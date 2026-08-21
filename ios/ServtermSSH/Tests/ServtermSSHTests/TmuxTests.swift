import Foundation
import Testing
@testable import ServtermSSH

@Suite("tmux commands")
struct TmuxCommandTests {
    @Test("every tmux command runs through a login shell")
    func loginShell() {
        // A non-interactive ssh command on macOS has a PATH without
        // /opt/homebrew/bin, so a bare tmux is not found. A login shell
        // reads the profile and finds it on every host.
        #expect(TmuxCommand.loginShell("tmux ls") == "\"$SHELL\" -lc 'tmux ls'")
    }

    @Test("the attach command reattaches one named session")
    func attach() {
        #expect(TmuxCommand.attach(session: "servterm-mobile", tmux: "/usr/bin/tmux")
            == "/usr/bin/tmux new -A -s servterm-mobile")
        #expect(TmuxCommand.attach(session: "work-1", tmux: "/usr/bin/tmux")
            == "/usr/bin/tmux new -A -s work-1")
    }

    @Test("the list command asks for the fields that the app parses")
    func list() {
        let command = TmuxCommand.listSessions(tmux: "/usr/bin/tmux")
        #expect(command.contains("tmux list-sessions -F"))
        #expect(command.contains("#{session_name}"))
        #expect(command.contains("#{session_windows}"))
        #expect(command.contains("#{session_attached}"))
        #expect(command.contains("#{session_created}"))
        #expect(command.contains("#{session_activity}"))
        #expect(command.hasPrefix("/usr/bin/tmux list-sessions"))
    }

    @Test("the kill command names one session")
    func kill() {
        #expect(TmuxCommand.kill(session: "old", tmux: "/usr/bin/tmux")
            == "/usr/bin/tmux kill-session -t old")
        #expect(TmuxCommand.kill(session: "bad name", tmux: "/usr/bin/tmux") == nil)
    }

    @Test("tmux refuses a name with a space, a colon or a dot, so the app does too")
    func nameRules() {
        #expect(TmuxSessionName.isValid("servterm-mobile"))
        #expect(TmuxSessionName.isValid("work_2"))
        #expect(TmuxSessionName.isValid("a"))
        #expect(TmuxSessionName.isValid("bad name") == false)
        #expect(TmuxSessionName.isValid("bad:name") == false)
        #expect(TmuxSessionName.isValid("bad.name") == false)
        #expect(TmuxSessionName.isValid("") == false)
        #expect(TmuxSessionName.isValid(String(repeating: "a", count: 60)) == false)
    }

    @Test("the name rule explains itself, for the screen")
    func nameHint() {
        #expect(TmuxSessionName.rule.contains("space"))
        #expect(TmuxSessionName.rule.contains("colon"))
        #expect(TmuxSessionName.rule.contains("dot"))
    }
}

@Suite("tmux session list")
struct TmuxSessionListTests {
    private let created = 1_787_280_000
    private let activity = 1_787_285_463

    private func line(_ name: String, windows: Int = 2, attached: Int = 0) -> String {
        "\(name)\t\(windows)\t\(attached)\t\(created)\t\(activity)"
    }

    @Test("one line becomes one session")
    func parseOne() throws {
        let sessions = TmuxSession.parse(line("servterm-mobile", windows: 3, attached: 1))
        #expect(sessions.count == 1)
        let session = try #require(sessions.first)
        #expect(session.name == "servterm-mobile")
        #expect(session.windows == 3)
        #expect(session.isAttached)
        #expect(session.created == Date(timeIntervalSince1970: TimeInterval(created)))
        #expect(session.lastActivity == Date(timeIntervalSince1970: TimeInterval(activity)))
    }

    @Test("a name with a dash survives the parse")
    func dashedName() {
        #expect(TmuxSession.parse(line("build-box-2")).first?.name == "build-box-2")
    }

    @Test("many lines keep their order")
    func parseMany() {
        let output = [line("one"), line("two", attached: 1), line("three")].joined(separator: "\n")
        #expect(TmuxSession.parse(output).map(\.name) == ["one", "two", "three"])
        #expect(TmuxSession.parse(output).map(\.isAttached) == [false, true, false])
    }

    @Test("a blank line and a short line are skipped, never half read")
    func skipsBadLines() {
        let output = [line("one"), "", "broken\tline", "  "].joined(separator: "\n")
        #expect(TmuxSession.parse(output).map(\.name) == ["one"])
    }

    @Test("no tmux server yet is an empty list, not a failure")
    func noServerRunning() {
        // This is the exact text that tmux prints on a host where nobody
        // started a session yet. It exits non-zero, and it means empty.
        let stderr = "error connecting to /tmp/tmux-0/default (No such file or directory)"
        let result = TmuxSessionList.read(stdout: "", stderr: stderr, exitStatus: 1)
        #expect(result == .sessions([]))
    }

    @Test("the other empty message from tmux means the same")
    func noServerRunningAlternative() {
        let result = TmuxSessionList.read(
            stdout: "", stderr: "no server running on /tmp/tmux-1000/default", exitStatus: 1)
        #expect(result == .sessions([]))
    }

    @Test("a real failure keeps its reason")
    func realFailure() {
        let result = TmuxSessionList.read(
            stdout: "", stderr: "zsh:1: command not found: tmux", exitStatus: 127)
        #expect(result == .failed("zsh:1: command not found: tmux"))
    }

    @Test("a failure with no message still says something")
    func failureWithoutText() {
        let result = TmuxSessionList.read(stdout: "", stderr: "   ", exitStatus: 2)
        #expect(result == .failed("tmux ended with status 2"))
    }

    @Test("a good exit with no line is an empty list")
    func emptySuccess() {
        #expect(TmuxSessionList.read(stdout: "\n", stderr: "", exitStatus: 0) == .sessions([]))
    }

    @Test("a good exit parses its lines")
    func successWithLines() {
        let result = TmuxSessionList.read(stdout: line("one"), stderr: "", exitStatus: 0)
        guard case .sessions(let sessions) = result else {
            Issue.record("the read did not return sessions")
            return
        }
        #expect(sessions.map(\.name) == ["one"])
    }

    @Test("the age of a session comes from its last activity")
    func age() {
        let session = TmuxSession.parse(line("one"))[0]
        let now = Date(timeIntervalSince1970: TimeInterval(activity + 90))
        #expect(session.idleSeconds(now: now) == 90)
        // A clock that runs backwards must not give a negative age.
        #expect(session.idleSeconds(now: Date(timeIntervalSince1970: 0)) == 0)
    }
}
