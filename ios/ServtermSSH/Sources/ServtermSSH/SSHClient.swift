import Crypto
import Foundation
import NIOCore
import NIOPosix
import NIOSSH

/// SSHEvent is everything that one session reports to the screen.
public enum SSHEvent: Sendable {
    case state(SessionState)
    case output([UInt8])
}

/// SSHConnecting hides the socket, so a screen can be driven by a fake in
/// a test. The app uses NIOSSHClient.
public protocol SSHConnecting: Sendable {
    func open(_ request: SSHRequest) -> AsyncStream<SSHEvent>
    func send(_ bytes: [UInt8]) async
    func resize(columns: Int, rows: Int) async
    func close() async
}

/// SSHRequest is one connection.
public struct SSHRequest: Sendable {
    public let host: String
    public let port: Int
    public let user: String
    public let identity: SSHIdentity
    public let plan: SessionPlan
    public let columns: Int
    public let rows: Int

    public init(
        host: String, port: Int = 22, user: String, identity: SSHIdentity,
        plan: SessionPlan, columns: Int, rows: Int
    ) {
        self.host = host
        self.port = port
        self.user = user
        self.identity = identity
        self.plan = plan
        self.columns = columns
        self.rows = rows
    }
}

/// SSHClientError names the failures that the screen shows.
public enum SSHClientError: Error, Equatable {
    case hostKeyChanged(String)
    case authenticationFailed
    case transport(String)
}

/// NIOSSHClient speaks SSH to the host's own sshd. It performs no command
/// of its own: it opens a shell, or the tmux attach command, and passes
/// bytes both ways.
public actor NIOSSHClient: SSHConnecting {
    private let group: EventLoopGroup
    private let checker: HostKeyChecker
    private var channel: Channel?
    private var child: Channel?
    private var continuation: AsyncStream<SSHEvent>.Continuation?

    public init(checker: HostKeyChecker, group: EventLoopGroup? = nil) {
        self.checker = checker
        self.group = group ?? MultiThreadedEventLoopGroup(numberOfThreads: 1)
    }

    public nonisolated func open(_ request: SSHRequest) -> AsyncStream<SSHEvent> {
        AsyncStream { continuation in
            Task { await self.start(request, continuation: continuation) }
            continuation.onTermination = { _ in Task { await self.close() } }
        }
    }

    private func start(_ request: SSHRequest, continuation: AsyncStream<SSHEvent>.Continuation) async {
        self.continuation = continuation
        continuation.yield(.state(.connecting))
        do {
            let authWatcher = AuthWatcher()
            let checker = self.checker
            let bootstrap = ClientBootstrap(group: group)
                .channelInitializer { channel in
                    channel.pipeline.addHandlers(
                        SSHPipeline.clientHandlers(
                            host: request.host, user: request.user, key: request.identity.nioKey,
                            checker: checker, watcher: authWatcher, allocator: channel.allocator))
                }
                .connectTimeout(.seconds(10))
            let channel = try await bootstrap.connect(host: request.host, port: request.port).get()
            self.channel = channel

            // The connect future resolves when the TCP socket opens, which
            // is long before SSH authenticates. Waiting for the success
            // event keeps a refused key from looking like a hang.
            try await authWatcher.waitForAuthentication(on: channel)

            let child = try await openSession(on: channel, request: request)
            self.child = child
            // Every host runs tmux, so the app always attaches: the state
            // says reattached when the session was already there.
            continuation.yield(.state(.connected(reattached: true)))

            try await child.closeFuture.get()
            continuation.yield(.state(.disconnected(reason: "the host closed the session")))
        } catch let error as SSHClientError {
            switch error {
            case .hostKeyChanged(let warning):
                continuation.yield(.state(.refused(warning: warning)))
            case .authenticationFailed:
                continuation.yield(
                    .state(.disconnected(reason: "the host refused the key of this phone")))
            case .transport(let detail):
                continuation.yield(.state(.disconnected(reason: detail)))
            }
        } catch {
            continuation.yield(.state(.disconnected(reason: Self.reason(error))))
        }
        continuation.finish()
    }

    /// openSession asks for a terminal and then for the shell or the tmux
    /// command. Both requests are the standard SSH channel requests.
    private func openSession(on channel: Channel, request: SSHRequest) async throws -> Channel {
        let promise = channel.eventLoop.makePromise(of: Channel.self)
        let output = OutputHandler { [weak self] bytes in
            Task { await self?.emit(bytes) }
        }
        // createChannel is NOT thread safe: the library says it may only be
        // called on the channel. Calling it from a task thread let the
        // request go missing.
        channel.pipeline.handler(type: NIOSSHHandler.self).whenComplete { outcome in
            switch outcome {
            case .failure(let error):
                promise.fail(error)
            case .success(let handler):
                handler.createChannel(promise) { child, type in
                    guard type == .session else {
                        return child.eventLoop.makeFailedFuture(
                            SSHClientError.transport("the host opened the wrong channel"))
                    }
                    return child.eventLoop.makeCompletedFuture {
                        try child.pipeline.syncOperations.addHandler(output)
                    }
                }
            }
        }
        channel.eventLoop.scheduleTask(in: .seconds(20)) {
            promise.fail(SSHClientError.transport("the host did not open a shell channel in time"))
        }
        let child = try await promise.futureResult.get()

        let pty = SSHChannelRequestEvent.PseudoTerminalRequest(
            wantReply: true,
            term: "xterm-256color",
            terminalCharacterWidth: request.columns,
            terminalRowHeight: request.rows,
            terminalPixelWidth: 0,
            terminalPixelHeight: 0,
            terminalModes: SSHTerminalModes([:]))
        try await child.triggerUserOutboundEvent(pty)

        try await child.triggerUserOutboundEvent(
            SSHChannelRequestEvent.ExecRequest(command: request.plan.command, wantReply: true))
        return child
    }

    private func emit(_ bytes: [UInt8]) {
        continuation?.yield(.output(bytes))
    }

    public func send(_ bytes: [UInt8]) async {
        guard let child, !bytes.isEmpty else { return }
        var buffer = child.allocator.buffer(capacity: bytes.count)
        buffer.writeBytes(bytes)
        try? await child.writeAndFlush(SSHChannelData(type: .channel, data: .byteBuffer(buffer))).get()
    }

    public func resize(columns: Int, rows: Int) async {
        guard let child else { return }
        let event = SSHChannelRequestEvent.WindowChangeRequest(
            terminalCharacterWidth: columns,
            terminalRowHeight: rows,
            terminalPixelWidth: 0,
            terminalPixelHeight: 0)
        try? await child.triggerUserOutboundEvent(event)
    }

    /// close drops the socket only. The tmux session on the host keeps
    /// running, which is the whole point of the design.
    public func close() async {
        try? await child?.close().get()
        try? await channel?.close().get()
        child = nil
        channel = nil
        continuation?.finish()
        continuation = nil
    }

    static func reason(_ error: any Error) -> String {
        if let error = error as? SSHClientError, case .transport(let detail) = error { return detail }
        if let error = error as? NIOSSHError { return String(describing: error) }
        return error.localizedDescription
    }
}

/// SSHPipeline builds the handlers of one client connection, in the one
/// order that works.
///
/// NIOSSH reports a finished authentication with
/// context.fireUserInboundEventTriggered, and an inbound event walks
/// towards the TAIL of the pipeline. The watcher must therefore sit BEHIND
/// the SSH handler. With the watcher in front, it never hears the event:
/// sshd accepts the key and opens a session, the app waits for a signal
/// that can never reach it, and nothing is ever asked of that session.
enum SSHPipeline {
    static func clientHandlers(
        host: String,
        user: String,
        key: NIOSSHPrivateKey,
        checker: HostKeyChecker,
        watcher: AuthWatcher,
        allocator: ByteBufferAllocator
    ) -> [ChannelHandler] {
        let delegate = PublicKeyAuthDelegate(
            user: user, key: key, onRefused: { [weak watcher] in watcher?.refuse() })
        let validator = HostKeyValidator(host: host, checker: checker)
        return [
            NIOSSHHandler(
                role: .client(.init(userAuthDelegate: delegate, serverAuthDelegate: validator)),
                allocator: allocator,
                inboundChildChannelInitializer: nil),
            watcher,
        ]
    }
}

/// AuthWatcher tells the client when the user authentication finished. A
/// closed connection before that means the host refused the key.
final class AuthWatcher: ChannelInboundHandler, @unchecked Sendable {
    typealias InboundIn = Any

    private let lock = NSLock()
    private var promise: EventLoopPromise<Void>?
    private var authenticated = false

    /// hasAuthenticated says whether the event ever arrived here.
    var hasAuthenticated: Bool { lock.withLock { authenticated } }

    func userInboundEventTriggered(context: ChannelHandlerContext, event: Any) {
        if event is UserAuthSuccessEvent {
            lock.withLock {
                authenticated = true
                promise?.succeed(())
            }
        }
        context.fireUserInboundEventTriggered(event)
    }

    /// refuse marks the key as refused. The host asks for another method
    /// only when it did not accept the one already offered.
    func refuse() {
        lock.withLock { promise?.fail(SSHClientError.authenticationFailed) }
    }

    /// waitForAuthentication finishes when the host accepts the key, and
    /// fails when the host refuses it, when the connection closes, or when
    /// the host says nothing at all.
    func waitForAuthentication(on channel: Channel, timeout: TimeAmount = .seconds(20)) async throws {
        try await authentication(on: channel, timeout: timeout).get()
    }

    /// authentication is the same wait as a future, so a test can drive it
    /// on an embedded loop.
    func authentication(on channel: Channel, timeout: TimeAmount) -> EventLoopFuture<Void> {
        let promise = channel.eventLoop.makePromise(of: Void.self)
        let alreadyDone = lock.withLock { () -> Bool in
            if authenticated { return true }
            self.promise = promise
            return false
        }
        if alreadyDone {
            promise.succeed(())
        } else {
            // A promise that already succeeded ignores these failures, so a
            // normal close after authentication is harmless.
            channel.closeFuture.whenComplete { _ in
                promise.fail(SSHClientError.authenticationFailed)
            }
            channel.eventLoop.scheduleTask(in: timeout) {
                promise.fail(SSHClientError.transport("the host did not answer the key in time"))
            }
        }
        return promise.futureResult
    }
}

/// HostKeyValidator refuses a changed host key. There is no way past it.
final class HostKeyValidator: NIOSSHClientServerAuthenticationDelegate, @unchecked Sendable {
    private let host: String
    private let checker: HostKeyChecker

    init(host: String, checker: HostKeyChecker) {
        self.host = host
        self.checker = checker
    }

    func validateHostKey(hostKey: NIOSSHPublicKey, validationCompletePromise: EventLoopPromise<Void>) {
        guard let fingerprint = SSHFingerprint.of(hostKey) else {
            validationCompletePromise.fail(SSHClientError.transport("the host key is not readable"))
            return
        }
        let decision = checker.decide(host: host, fingerprint: fingerprint)
        switch decision {
        case .known:
            validationCompletePromise.succeed(())
        case .firstUse:
            checker.pin(host: host, fingerprint: fingerprint)
            validationCompletePromise.succeed(())
        case .changed:
            validationCompletePromise.fail(
                SSHClientError.hostKeyChanged(decision.warning(host: host, offered: fingerprint)))
        }
    }
}

/// PublicKeyAuthDelegate offers the key of this phone, and nothing else.
/// It never offers a password, because the app stores none.
final class PublicKeyAuthDelegate: NIOSSHClientUserAuthenticationDelegate, @unchecked Sendable {
    private let user: String
    private let key: NIOSSHPrivateKey
    private let onRefused: () -> Void
    private var offered = false

    init(user: String, key: NIOSSHPrivateKey, onRefused: @escaping () -> Void) {
        self.user = user
        self.key = key
        self.onRefused = onRefused
    }

    func nextAuthenticationType(
        availableMethods: NIOSSHAvailableUserAuthenticationMethods,
        nextChallengePromise: EventLoopPromise<NIOSSHUserAuthenticationOffer?>
    ) {
        guard availableMethods.contains(.publicKey), !offered else {
            // A second call means the host did not accept the key. The app
            // stores no password, so there is nothing else to offer.
            onRefused()
            nextChallengePromise.succeed(nil)
            return
        }
        offered = true
        nextChallengePromise.succeed(
            NIOSSHUserAuthenticationOffer(
                username: user, serviceName: "", offer: .privateKey(.init(privateKey: key))))
    }
}

/// OutputHandler passes the bytes of the shell to the screen.
final class OutputHandler: ChannelInboundHandler, @unchecked Sendable {
    typealias InboundIn = SSHChannelData

    private let onOutput: ([UInt8]) -> Void

    init(onOutput: @escaping ([UInt8]) -> Void) {
        self.onOutput = onOutput
    }

    func handlerAdded(context: ChannelHandlerContext) {
        // A shell that sends the end of input must not lose its last bytes.
        context.channel.setOption(ChannelOptions.allowRemoteHalfClosure, value: true).whenFailure {
            error in
            context.fireErrorCaught(error)
        }
    }

    func channelRead(context: ChannelHandlerContext, data: NIOAny) {
        let channelData = unwrapInboundIn(data)
        guard case .byteBuffer(let buffer) = channelData.data else { return }
        // A shell writes its errors on the second stream, and a reader
        // wants to see them in the terminal too.
        switch channelData.type {
        case .channel, .stdErr:
            onOutput(Array(buffer.readableBytesView))
        default:
            break
        }
    }
}
