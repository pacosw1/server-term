import Foundation
import Testing
@testable import ServtermSSH

@Suite("Session state and tmux")
struct SessionTests {
    // MARK: - the session plan

    @Test("a plan attaches to its own named session with the resolved tmux")
    func attachPlan() throws {
        let plan = try #require(SessionPlan.attach(session: "servterm-mobile", tmux: "/usr/bin/tmux"))
        #expect(plan.session == "servterm-mobile")
        #expect(plan.command == "/usr/bin/tmux new -A -s servterm-mobile")
        #expect(plan.note.contains("tmux"))
        #expect(plan.isPersistent)
    }

    @Test("a name that tmux would refuse makes no plan at all")
    func refusedName() {
        #expect(SessionPlan.attach(session: "bad name", tmux: "/usr/bin/tmux") == nil)
        #expect(SessionPlan.attach(session: "bad;name", tmux: "/usr/bin/tmux") == nil)
        #expect(SessionPlan.attach(session: "bad\"name", tmux: "/usr/bin/tmux") == nil)
        #expect(SessionPlan.attach(session: "", tmux: "/usr/bin/tmux") == nil)
    }

    // MARK: - the state machine

    @Test("a fresh session is idle and shows no terminal as live")
    func startsIdle() {
        let machine = SessionMachine()
        #expect(machine.state == .idle)
        #expect(machine.state.isLive == false)
    }

    @Test("the states follow the connection, and say when a session came back")
    func connectFlow() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        #expect(machine.state == .connecting)
        #expect(machine.state.isLive == false)
        machine.apply(.channelOpened(reattached: true))
        #expect(machine.state == .connected(reattached: true))
        #expect(machine.state.isLive)
        #expect(machine.state.label == "reattached")
    }

    @Test("a first connection says connected, not reattached")
    func freshConnection() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        machine.apply(.channelOpened(reattached: false))
        #expect(machine.state.label == "connected")
    }

    @Test("a drop names its reason and is never live")
    func dropped() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        machine.apply(.channelOpened(reattached: false))
        machine.apply(.disconnected(reason: "the tailnet dropped"))
        #expect(machine.state == .disconnected(reason: "the tailnet dropped"))
        #expect(machine.state.isLive == false)
        #expect(machine.state.label.contains("disconnected"))
    }

    @Test("a refused host key ends in its own state, not in a plain drop")
    func refusedHostKey() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        machine.apply(.hostKeyRefused(warning: "the key changed"))
        #expect(machine.state == .refused(warning: "the key changed"))
        #expect(machine.state.isLive == false)
    }

    @Test("a reconnect after a drop starts from connecting again")
    func reconnect() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        machine.apply(.disconnected(reason: "no route"))
        machine.apply(.connectStarted)
        #expect(machine.state == .connecting)
    }

    @Test("the app closing leaves the session alone on the host")
    func backgroundKeepsHostSession() {
        var machine = SessionMachine()
        machine.apply(.connectStarted)
        machine.apply(.channelOpened(reattached: false))
        machine.apply(.appLeftScreen)
        // The socket goes, the tmux session on the host does not.
        #expect(machine.state == .detached)
        #expect(machine.state.isLive == false)
        #expect(machine.state.label.contains("detached"))
    }
}
