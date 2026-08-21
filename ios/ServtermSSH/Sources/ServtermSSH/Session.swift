import Foundation

/// SessionPlan is the command that opens one session. Every host runs
/// tmux, so every session is persistent: the socket can drop and the work
/// keeps running on the host.
public struct SessionPlan: Sendable, Equatable {
    public let session: String
    public let command: String
    public let isPersistent: Bool
    public let note: String

    /// attach builds the plan for one named session, with the tmux binary
    /// that the app resolved for that host. It returns nil for a name that
    /// tmux would refuse.
    public static func attach(session: String, tmux: String) -> SessionPlan? {
        guard let command = TmuxCommand.attach(session: session, tmux: tmux) else { return nil }
        return SessionPlan(
            session: session,
            command: command,
            isPersistent: true,
            note: """
                This session lives in tmux on the host. Closing the app \
                leaves the work running, and the app attaches to it again.
                """)
    }

    /// plainShell is the last case: a host where the app found no tmux at
    /// all. The shell works, the session does not survive, and the screen
    /// says why.
    public static func plainShell(reason: String) -> SessionPlan {
        SessionPlan(
            session: "",
            command: "\"$SHELL\" -l",
            isPersistent: false,
            note: reason + ", so this session ends when the connection ends.")
    }
}

/// SessionState is what the screen shows. A dead terminal is never shown
/// as a live one.
public enum SessionState: Sendable, Equatable {
    case idle
    case connecting
    case connected(reattached: Bool)
    /// detached means the app left the screen. The session keeps running
    /// on the host.
    case detached
    case disconnected(reason: String)
    /// refused means the host key changed. Only the user can clear it.
    case refused(warning: String)

    public var isLive: Bool {
        if case .connected = self { return true }
        return false
    }

    public var label: String {
        switch self {
        case .idle: return "not connected"
        case .connecting: return "connecting"
        case .connected(let reattached): return reattached ? "reattached" : "connected"
        case .detached: return "detached, still running on the host"
        case .disconnected(let reason): return "disconnected: " + reason
        case .refused: return "refused: the host key changed"
        }
    }
}

/// SessionEvent is everything that can move the state.
public enum SessionEvent: Sendable, Equatable {
    case connectStarted
    case channelOpened(reattached: Bool)
    case appLeftScreen
    case disconnected(reason: String)
    case hostKeyRefused(warning: String)
}

/// SessionMachine holds the state. It is a plain value, so a test can walk
/// every path without a socket.
public struct SessionMachine: Sendable, Equatable {
    public private(set) var state: SessionState = .idle

    public init() {}

    public mutating func apply(_ event: SessionEvent) {
        switch event {
        case .connectStarted:
            state = .connecting
        case .channelOpened(let reattached):
            state = .connected(reattached: reattached)
        case .appLeftScreen:
            state = .detached
        case .disconnected(let reason):
            state = .disconnected(reason: reason)
        case .hostKeyRefused(let warning):
            state = .refused(warning: warning)
        }
    }
}
