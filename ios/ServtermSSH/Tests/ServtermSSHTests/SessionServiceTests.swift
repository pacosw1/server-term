import Foundation
import Testing
@testable import ServtermSSH

/// FakeRunner answers each command from a script, and records what it was
/// asked to run.
private final class FakeRunner: SSHRunning, @unchecked Sendable {
    private let lock = NSLock()
    private var answers: [String: CommandResult]
    private(set) var commands: [String] = []
    var failure: (any Error)?

    init(_ answers: [String: CommandResult] = [:]) { self.answers = answers }

    func run(_ request: SSHRequest, command: String) async throws -> CommandResult {
        lock.withLock { commands.append(command) }
        if let failure { throw failure }
        let match = lock.withLock { answers.first { command.contains($0.key) }?.value }
        return match ?? CommandResult(stdout: "", stderr: "", exitStatus: 0)
    }
}

private struct FixedLocator: TmuxLocating {
    let path: String?
    var error: (any Error)?
    func locate(_ request: SSHRequest) async throws -> String? {
        if let error { throw error }
        return path
    }
}

@Suite("Session service")
struct SessionServiceTests {
    private func request() throws -> SSHRequest {
        SSHRequest(
            host: "10.0.0.1", user: "root",
            identity: try SSHIdentity.generateInMemory(comment: "test"),
            plan: SessionPlan.plainShell(reason: "probe"), columns: 80, rows: 24)
    }

    @Test("a host with no tmux server lists as empty, not as a failure")
    func emptyList() async throws {
        let runner = FakeRunner([
            "list-sessions": CommandResult(
                stdout: "",
                stderr: "error connecting to /tmp/tmux-0/default (No such file or directory)",
                exitStatus: 1)
        ])
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        #expect(await service.list(try request()) == .sessions([]))
    }

    @Test("the list sorts the newest activity first")
    func sortedList() async throws {
        let runner = FakeRunner([
            "list-sessions": CommandResult(
                stdout: "old\t1\t0\t100\t100\nnew\t2\t1\t200\t900\n", stderr: "", exitStatus: 0)
        ])
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        guard case .sessions(let sessions) = await service.list(try request()) else {
            Issue.record("the list failed")
            return
        }
        #expect(sessions.map(\.name) == ["new", "old"])
    }

    @Test("a real failure keeps its reason")
    func failedList() async throws {
        let runner = FakeRunner([
            "list-sessions": CommandResult(stdout: "", stderr: "command not found: tmux", exitStatus: 127)
        ])
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        #expect(await service.list(try request()) == .failed("command not found: tmux"))
    }

    @Test("making a session uses the detached form, never the attach form")
    func createUsesDetachedForm() async throws {
        let runner = FakeRunner()
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        let error = await service.create(name: "work", request: try request())
        #expect(error == nil)
        let command = try #require(runner.commands.last)
        #expect(command == "/usr/bin/tmux new-session -d -s work")
        #expect(command.contains("new -A") == false, "the attach form needs a terminal, which this channel has none of")
    }

    @Test("a name that tmux would refuse never reaches the host")
    func refusesBadName() async throws {
        let runner = FakeRunner()
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        let error = await service.create(name: "bad name", request: try request())
        #expect(error == TmuxSessionName.rule)
        #expect(runner.commands.isEmpty)
    }

    @Test("a failed command reports the message of the host")
    func failedCommand() async throws {
        let runner = FakeRunner([
            "kill-session": CommandResult(stdout: "", stderr: "can't find session: gone", exitStatus: 1)
        ])
        let service = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        #expect(await service.kill(name: "gone", request: try request()) == "can't find session: gone")
    }

    @Test("the attach plan carries the resolved tmux, and a host without one still gets a shell")
    func plans() async throws {
        let runner = FakeRunner()
        let withTmux = TmuxSessionService(runner: runner, locator: FixedLocator(path: "/usr/bin/tmux"))
        let plan = await withTmux.plan(session: "work", request: try request())
        #expect(plan.command == "/usr/bin/tmux new -A -s work")
        #expect(plan.isPersistent)

        let without = TmuxSessionService(runner: runner, locator: FixedLocator(path: nil))
        let fallback = await without.plan(session: "work", request: try request())
        #expect(fallback.isPersistent == false)
        #expect(fallback.note.contains("no tmux"))
    }

    @Test("a probe that cannot reach the host says so, and does not blame tmux")
    func unreachableHost() async throws {
        let locator = FixedLocator(path: nil, error: SSHClientError.authenticationFailed)
        let service = TmuxSessionService(runner: FakeRunner(), locator: locator)
        #expect(await service.list(try request()) == .failed("the host refused the key of this phone"))
        let plan = await service.plan(session: "work", request: try request())
        #expect(plan.note.contains("could not ask"))
    }
}
