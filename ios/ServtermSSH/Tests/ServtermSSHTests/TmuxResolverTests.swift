import Foundation
import Testing
@testable import ServtermSSH

@Suite("Finding tmux")
struct TmuxResolverTests {
    @Test("the probe tries the plain name, then a login shell, then the known paths")
    func probeOrder() {
        let probe = TmuxResolver.probeCommand
        let direct = try! #require(probe.range(of: "command -v tmux"))
        let login = try! #require(probe.range(of: "\"$SHELL\" -lc"))
        let known = try! #require(probe.range(of: "/opt/homebrew/bin/tmux"))
        #expect(direct.lowerBound < login.lowerBound)
        #expect(login.lowerBound < known.lowerBound)
        #expect(probe.contains("/usr/bin/tmux"))
        #expect(probe.contains("/usr/local/bin/tmux"))
    }

    @Test("the probe answer gives the absolute path")
    func parsePath() {
        #expect(TmuxResolver.path(fromProbe: "/opt/homebrew/bin/tmux\n") == "/opt/homebrew/bin/tmux")
        #expect(TmuxResolver.path(fromProbe: "  /usr/bin/tmux  ") == "/usr/bin/tmux")
    }

    @Test("the first path wins when the probe prints more than one")
    func parseFirstPath() {
        #expect(TmuxResolver.path(fromProbe: "/usr/bin/tmux\n/opt/homebrew/bin/tmux\n")
            == "/usr/bin/tmux")
    }

    @Test("a host with no tmux gives no path, and no invented one")
    func parseMissing() {
        #expect(TmuxResolver.path(fromProbe: "") == nil)
        #expect(TmuxResolver.path(fromProbe: "\n\n") == nil)
        #expect(TmuxResolver.path(fromProbe: "zsh:1: command not found: tmux") == nil)
        // A relative answer is not a path the app will run.
        #expect(TmuxResolver.path(fromProbe: "tmux") == nil)
    }

    @Test("every command uses the resolved absolute path")
    func commandsUsePath() {
        let tmux = "/opt/homebrew/bin/tmux"
        #expect(TmuxCommand.attach(session: "work", tmux: tmux)
            == "/opt/homebrew/bin/tmux new -A -s work")
        #expect(TmuxCommand.kill(session: "work", tmux: tmux)
            == "/opt/homebrew/bin/tmux kill-session -t work")
        #expect(TmuxCommand.listSessions(tmux: tmux).hasPrefix("/opt/homebrew/bin/tmux list-sessions -F"))
    }

    @Test("a bad session name still makes no command")
    func badNames() {
        #expect(TmuxCommand.attach(session: "bad name", tmux: "/usr/bin/tmux") == nil)
        #expect(TmuxCommand.kill(session: "bad:name", tmux: "/usr/bin/tmux") == nil)
    }

    @Test("the cache keeps one path for each host")
    func cache() {
        let cache = MemoryTmuxCache()
        #expect(cache.path(forHost: "a") == nil)
        cache.setPath("/usr/bin/tmux", forHost: "a")
        #expect(cache.path(forHost: "a") == "/usr/bin/tmux")
        #expect(cache.path(forHost: "b") == nil)
        cache.setPath("/opt/homebrew/bin/tmux", forHost: "b")
        #expect(cache.path(forHost: "a") == "/usr/bin/tmux")
        cache.forget(host: "a")
        #expect(cache.path(forHost: "a") == nil)
    }

    @Test("a host with no tmux at all gets a plain shell, and the reason")
    func fallbackPlan() {
        let plan = SessionPlan.plainShell(reason: "the app found no tmux on this host")
        #expect(plan.command == "\"$SHELL\" -l")
        #expect(plan.isPersistent == false)
        #expect(plan.note.contains("no tmux"))
    }

    @Test("a resolved host gets the persistent plan")
    func attachPlanWithPath() throws {
        let plan = try #require(SessionPlan.attach(session: "work", tmux: "/usr/bin/tmux"))
        #expect(plan.command == "/usr/bin/tmux new -A -s work")
        #expect(plan.isPersistent)
    }
}
