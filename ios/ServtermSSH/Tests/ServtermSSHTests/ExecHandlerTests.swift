import Foundation
import NIOCore
import NIOEmbedded
import NIOSSH
import Testing
@testable import ServtermSSH

/// These tests drive the real exec handler through an EmbeddedChannel, the
/// way a host drives it: a request, output in pieces, an exit status and a
/// close. The earlier tests never exercised this exchange, so they passed
/// while the app hung on every command.
///
/// No test here waits on the promise. An EmbeddedChannel runs every step at
/// once on the calling thread, so the answer is already there when the last
/// event returns. A handler that never answers therefore fails in about a
/// second with a named message, instead of hanging the run.
@Suite("Exec channel")
struct ExecHandlerTests {
    private func data(_ text: String, type: SSHChannelData.DataType, allocator: ByteBufferAllocator)
        -> SSHChannelData
    {
        var buffer = allocator.buffer(capacity: text.utf8.count)
        buffer.writeString(text)
        return SSHChannelData(type: type, data: .byteBuffer(buffer))
    }

    @Test("the handler asks for the command as soon as the channel opens")
    func sendsExecRequest() throws {
        let channel = EmbeddedChannel()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        let events = EventRecorder()
        try channel.pipeline.syncOperations.addHandler(events)
        try channel.pipeline.syncOperations.addHandler(
            ExecHandler(command: "echo servterm-exec-ok", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        let request = try #require(events.outbound.first as? SSHChannelRequestEvent.ExecRequest)
        #expect(request.command == "echo servterm-exec-ok")
        #expect(request.wantReply)
        _ = try? channel.finish()
    }

    @Test("output that arrives in two pieces is joined, with the error stream apart")
    func joinsChunks() throws {
        let channel = EmbeddedChannel()
        let answer = Answer()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        answer.listen(to: promise)
        try channel.pipeline.syncOperations.addHandler(ExecHandler(command: "x", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        let allocator = channel.allocator
        try channel.writeInbound(data("servterm-", type: .channel, allocator: allocator))
        try channel.writeInbound(data("exec-ok\n", type: .channel, allocator: allocator))
        try channel.writeInbound(data("a warning\n", type: .stdErr, allocator: allocator))
        channel.pipeline.fireUserInboundEventTriggered(SSHChannelRequestEvent.ExitStatus(exitStatus: 0))
        channel.pipeline.fireChannelInactive()

        let result = try answer.value()
        #expect(result.stdout == "servterm-exec-ok\n")
        #expect(result.stderr == "a warning\n")
        #expect(result.exitStatus == 0)
        _ = try? channel.finish()
    }

    @Test("an end of input finishes the command, without waiting for a close")
    func finishesOnEOF() throws {
        let channel = EmbeddedChannel()
        let answer = Answer()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        answer.listen(to: promise)
        try channel.pipeline.syncOperations.addHandler(ExecHandler(command: "x", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        try channel.writeInbound(data("/usr/bin/tmux\n", type: .channel, allocator: channel.allocator))
        channel.pipeline.fireUserInboundEventTriggered(SSHChannelRequestEvent.ExitStatus(exitStatus: 0))
        channel.pipeline.fireUserInboundEventTriggered(ChannelEvent.inputClosed)

        let result = try answer.value()
        #expect(result.stdout == "/usr/bin/tmux\n")
        #expect(result.exitStatus == 0)
        _ = try? channel.finish()
    }

    @Test("a close with no exit status is a failure, never a quiet success")
    func closeWithoutStatus() throws {
        let channel = EmbeddedChannel()
        let answer = Answer()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        answer.listen(to: promise)
        try channel.pipeline.syncOperations.addHandler(ExecHandler(command: "x", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        try channel.writeInbound(data("partial", type: .channel, allocator: channel.allocator))
        channel.pipeline.fireChannelInactive()

        let result = try answer.value()
        #expect(result.stdout == "partial")
        #expect(result.exitStatus == 255)
        _ = try? channel.finish()
    }

    @Test("a non-zero status survives, so tmux with no server still reads as empty")
    func keepsNonZeroStatus() throws {
        let channel = EmbeddedChannel()
        let answer = Answer()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        answer.listen(to: promise)
        try channel.pipeline.syncOperations.addHandler(ExecHandler(command: "x", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        try channel.writeInbound(
            data("error connecting to /tmp/tmux-0/default (No such file or directory)\n",
                 type: .stdErr, allocator: channel.allocator))
        channel.pipeline.fireUserInboundEventTriggered(SSHChannelRequestEvent.ExitStatus(exitStatus: 1))
        channel.pipeline.fireChannelInactive()

        let result = try answer.value()
        #expect(result.exitStatus == 1)
        #expect(TmuxSessionList.read(
            stdout: result.stdout, stderr: result.stderr, exitStatus: result.exitStatus) == .sessions([]))
        _ = try? channel.finish()
    }

    @Test("the result is delivered once, even when EOF and close both arrive")
    func deliversOnce() throws {
        let channel = EmbeddedChannel()
        let answer = Answer()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        answer.listen(to: promise)
        try channel.pipeline.syncOperations.addHandler(ExecHandler(command: "x", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/test")).wait()

        channel.pipeline.fireUserInboundEventTriggered(SSHChannelRequestEvent.ExitStatus(exitStatus: 0))
        channel.pipeline.fireUserInboundEventTriggered(ChannelEvent.inputClosed)
        channel.pipeline.fireChannelInactive()
        // A promise that is answered twice would trap, so reaching this
        // line at all is half the check; the count is the other half.
        #expect(try answer.value().exitStatus == 0)
        #expect(answer.count == 1)
        _ = try? channel.finish()
    }
}

/// Answer holds the result that the handler delivered. An EmbeddedChannel
/// runs every step at once, so the answer is there as soon as the last
/// event returns. Reading it instead of waiting on the promise means a
/// handler that never answers fails at once with a message, instead of
/// hanging the whole run.
private final class Answer: @unchecked Sendable {
    private var result: CommandResult?
    private(set) var count = 0

    func listen(to promise: EventLoopPromise<CommandResult>) {
        promise.futureResult.whenSuccess { [weak self] value in
            self?.result = value
            self?.count += 1
        }
    }

    /// value returns the result, or fails the test with a named message.
    func value(sourceLocation: SourceLocation = #_sourceLocation) throws -> CommandResult {
        try #require(
            result,
            "the exec handler never answered: the command would hang the app",
            sourceLocation: sourceLocation)
    }
}

/// EventRecorder keeps the outbound user events, so a test can see the
/// request that the handler sent.
private final class EventRecorder: ChannelOutboundHandler, @unchecked Sendable {
    typealias OutboundIn = Any

    private(set) var outbound: [Any] = []

    func triggerUserOutboundEvent(
        context: ChannelHandlerContext, event: Any, promise: EventLoopPromise<Void>?
    ) {
        outbound.append(event)
        promise?.succeed(())
    }
}
