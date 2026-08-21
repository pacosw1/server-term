import Foundation
import NIOCore
import NIOEmbedded
import NIOSSH
import Testing
@testable import ServtermSSH

/// tmux refuses to attach when its input is not a terminal, with
/// "open terminal failed: not a terminal". So the attach channel MUST get a
/// pseudo terminal before its command, and the channels that only read must
/// NOT get one, because a terminal would fold and colour the output that
/// the app parses.
@Suite("Terminal requests")
struct TerminalRequestTests {
    // MARK: - the attach path asks for a terminal first

    @Test("the attach path sends the terminal request before the command")
    func attachOrder() throws {
        let requests = SSHChannelRequests.attach(
            command: "/usr/bin/tmux new -A -s work", columns: 100, rows: 30)
        #expect(requests.count == 2)
        let pty = try #require(requests.first as? SSHChannelRequestEvent.PseudoTerminalRequest)
        let exec = try #require(requests.last as? SSHChannelRequestEvent.ExecRequest)
        #expect(pty.term == "xterm-256color")
        #expect(pty.terminalCharacterWidth == 100)
        #expect(pty.terminalRowHeight == 30)
        #expect(pty.wantReply, "a terminal request without a reply cannot fail loudly")
        #expect(exec.command == "/usr/bin/tmux new -A -s work")
        #expect(exec.wantReply)
    }

    @Test("a host that refuses the terminal ends the attempt with a named failure")
    func refusedTerminal() {
        let failure = SSHClientError.terminalRefused
        #expect(failure.message.contains("terminal"))
        // The failure must not be mistaken for a plain transport fault, so
        // the screen can say what really happened.
        #expect(failure != SSHClientError.transport("the host refused a terminal"))
    }

    // MARK: - the reading paths ask for no terminal

    @Test("the command channel sends the command and nothing else")
    func execSendsNoTerminal() throws {
        let channel = EmbeddedChannel()
        let recorder = RequestRecorder()
        let promise = channel.eventLoop.makePromise(of: CommandResult.self)
        try channel.pipeline.syncOperations.addHandler(recorder)
        try channel.pipeline.syncOperations.addHandler(
            ExecHandler(command: "/usr/bin/tmux list-sessions", promise: promise))
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/pty")).wait()

        #expect(recorder.events.count == 1, "a reading channel must send one request only")
        #expect(recorder.events.first is SSHChannelRequestEvent.ExecRequest)
        #expect(
            recorder.events.contains { $0 is SSHChannelRequestEvent.PseudoTerminalRequest } == false,
            "a terminal would fold and colour the output that the app parses")
        channel.pipeline.fireChannelInactive()
        _ = try? channel.finish()
    }

    // MARK: - the commands themselves

    @Test("making a session uses the detached form, which needs no terminal")
    func createCommand() {
        #expect(TmuxCommand.create(session: "work", tmux: "/usr/bin/tmux")
            == "/usr/bin/tmux new-session -d -s work")
        #expect(TmuxCommand.create(session: "bad name", tmux: "/usr/bin/tmux") == nil)
    }

    @Test("the attach form stays for the interactive channel only")
    func attachCommand() {
        #expect(TmuxCommand.attach(session: "work", tmux: "/usr/bin/tmux")
            == "/usr/bin/tmux new -A -s work")
    }

    @Test("the two forms are not the same, which is the whole point")
    func formsDiffer() {
        let attach = TmuxCommand.attach(session: "work", tmux: "/usr/bin/tmux")
        let create = TmuxCommand.create(session: "work", tmux: "/usr/bin/tmux")
        #expect(attach != create)
        #expect(create?.contains("-d") == true, "the detached form must carry -d")
        #expect(attach?.contains("-d") == false, "the attach form must not detach")
    }
}

/// RequestRecorder keeps every outbound channel request, so a test can see
/// exactly what one channel asks the host for.
final class RequestRecorder: ChannelOutboundHandler, @unchecked Sendable {
    typealias OutboundIn = Any

    private let lock = NSLock()
    private var stored: [Any] = []

    var events: [Any] { lock.withLock { stored } }

    func triggerUserOutboundEvent(
        context: ChannelHandlerContext, event: Any, promise: EventLoopPromise<Void>?
    ) {
        lock.withLock { stored.append(event) }
        promise?.succeed(())
    }
}
