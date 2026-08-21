import Foundation
import NIOCore
import NIOPosix
import NIOSSH

/// CommandResult is what one non-interactive command left behind.
public struct CommandResult: Sendable, Equatable {
    public let stdout: String
    public let stderr: String
    public let exitStatus: Int32
}

/// SSHRunning hides the command channel, so a screen can be driven by a
/// fake in a test.
public protocol SSHRunning: Sendable {
    func run(_ request: SSHRequest, command: String) async throws -> CommandResult
}

/// SSHCommandRunner runs one short command and collects its output. It is
/// used for the session list and for making or killing a session. It runs
/// only the commands that this app builds, never text from a host.
public struct SSHCommandRunner: SSHRunning {
    private let checker: HostKeyChecker
    private let group: EventLoopGroup
    private let timeout: TimeAmount

    public init(
        checker: HostKeyChecker, group: EventLoopGroup? = nil, timeout: TimeAmount = .seconds(20)
    ) {
        self.checker = checker
        self.group = group ?? MultiThreadedEventLoopGroup(numberOfThreads: 1)
        self.timeout = timeout
    }

    public func run(_ request: SSHRequest, command: String) async throws -> CommandResult {
        let authWatcher = AuthWatcher()
        let bootstrap = ClientBootstrap(group: group)
            .channelInitializer { channel in
                channel.pipeline.addHandlers(
                    SSHPipeline.clientHandlers(
                        host: request.host, user: request.user, key: request.identity.nioKey,
                        checker: checker, watcher: authWatcher, allocator: channel.allocator))
            }
            .connectTimeout(.seconds(10))
        let channel = try await bootstrap.connect(host: request.host, port: request.port).get()
        do {
            let result = try await runCommand(command, on: channel, authWatcher: authWatcher)
            try? await channel.close()
            return result
        } catch {
            try? await channel.close()
            throw error
        }
    }

    private func runCommand(
        _ command: String, on channel: Channel, authWatcher: AuthWatcher
    ) async throws -> CommandResult {
        try await authWatcher.waitForAuthentication(on: channel)

        // createChannel is NOT thread safe: the library says it may only be
        // called on the channel. Calling it from a task thread let the
        // request go missing, which made every command hang: the resolver
        // read nothing, the session list never arrived, and creating a
        // session did nothing at all.
        let resultPromise = channel.eventLoop.makePromise(of: CommandResult.self)
        let childPromise = channel.eventLoop.makePromise(of: Channel.self)
        channel.pipeline.handler(type: NIOSSHHandler.self).whenComplete { outcome in
            switch outcome {
            case .failure(let error):
                childPromise.fail(error)
            case .success(let handler):
                handler.createChannel(childPromise) { child, type in
                    guard type == .session else {
                        return child.eventLoop.makeFailedFuture(
                            SSHClientError.transport("the host opened the wrong channel"))
                    }
                    return child.eventLoop.makeCompletedFuture {
                        try child.pipeline.syncOperations.addHandler(
                            ExecHandler(command: command, promise: resultPromise))
                    }
                }
            }
        }
        // A command that never answers must fail with a reason, not hang.
        channel.eventLoop.scheduleTask(in: timeout) {
            resultPromise.fail(SSHClientError.transport("the host did not answer the command in time"))
            childPromise.fail(SSHClientError.transport("the host did not open a command channel in time"))
        }
        _ = try await childPromise.futureResult.get()
        return try await resultPromise.futureResult.get()
    }
}

/// ExecHandler runs one command on one SSH channel and reports everything
/// it saw. It sends the request itself, keeps the two streams apart, and
/// finishes on the end of input or on the close, whichever the host sends.
final class ExecHandler: ChannelDuplexHandler, @unchecked Sendable {
    typealias InboundIn = SSHChannelData
    typealias InboundOut = ByteBuffer
    typealias OutboundIn = ByteBuffer
    typealias OutboundOut = SSHChannelData

    private let command: String
    private var promise: EventLoopPromise<CommandResult>?
    private var out = Data()
    private var err = Data()
    private var status: Int32 = -1

    init(command: String, promise: EventLoopPromise<CommandResult>) {
        self.command = command
        self.promise = promise
    }

    func handlerAdded(context: ChannelHandlerContext) {
        // Without this, the end of input closes the whole channel and the
        // last bytes can be lost.
        context.channel.setOption(ChannelOptions.allowRemoteHalfClosure, value: true).whenFailure {
            error in
            context.fireErrorCaught(error)
        }
    }

    func channelActive(context: ChannelHandlerContext) {
        context.triggerUserOutboundEvent(
            SSHChannelRequestEvent.ExecRequest(command: command, wantReply: true),
            promise: nil)
        context.fireChannelActive()
    }

    func channelRead(context: ChannelHandlerContext, data: NIOAny) {
        let channelData = unwrapInboundIn(data)
        guard case .byteBuffer(let buffer) = channelData.data else { return }
        let bytes = Data(buffer.readableBytesView)
        switch channelData.type {
        case .channel: out.append(bytes)
        case .stdErr: err.append(bytes)
        default: break
        }
    }

    func userInboundEventTriggered(context: ChannelHandlerContext, event: Any) {
        switch event {
        case let exit as SSHChannelRequestEvent.ExitStatus:
            status = Int32(exit.exitStatus)
        case ChannelEvent.inputClosed:
            // The host said it has nothing more to send. The command is
            // finished, whatever the close does next.
            finish()
        default:
            break
        }
        context.fireUserInboundEventTriggered(event)
    }

    func channelInactive(context: ChannelHandlerContext) {
        finish()
        context.fireChannelInactive()
    }

    func handlerRemoved(context: ChannelHandlerContext) {
        finish()
    }

    func errorCaught(context: ChannelHandlerContext, error: any Error) {
        promise?.fail(error)
        promise = nil
        context.close(promise: nil)
    }

    /// finish answers once. A channel that closed with no exit status is a
    /// failure, never a quiet success.
    private func finish() {
        guard let promise else { return }
        self.promise = nil
        promise.succeed(
            CommandResult(
                stdout: String(decoding: out, as: UTF8.self),
                stderr: String(decoding: err, as: UTF8.self),
                exitStatus: status < 0 ? 255 : status))
    }
}
