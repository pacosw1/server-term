import Foundation

/// TmuxSessionName holds the naming rule. tmux refuses a name with a
/// space, a colon or a dot, so the app refuses it first and says why.
public enum TmuxSessionName {
    public static let rule =
        "A session name takes letters, digits, a dash and an underscore. It cannot hold a space, a colon or a dot."

    public static func isValid(_ name: String) -> Bool {
        guard !name.isEmpty, name.count <= 40 else { return false }
        let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_"))
        return name.unicodeScalars.allSatisfy { allowed.contains($0) }
    }
}

/// TmuxCommand builds every command that the app runs on a host.
///
/// Each one goes through a login shell. A non-interactive ssh command on
/// macOS gets a PATH without /opt/homebrew/bin, so a bare tmux is not
/// found there; a login shell reads the profile and finds tmux on every
/// host. One rule for every host beats a cache of paths that can go stale.
public enum TmuxCommand {
    public static let defaultSession = "servterm-mobile"

    /// The fields that the app parses. The format is explicit, so the app
    /// never reads the human layout of tmux.
    static let listFormat =
        "#{session_name}\t#{session_windows}\t#{session_attached}\t#{session_created}\t#{session_activity}"

    public static func loginShell(_ command: String) -> String {
        "\"$SHELL\" -lc '" + command + "'"
    }

    /// attach runs the resolved tmux binary, so no command depends on the
    /// PATH of a non-interactive session.
    ///
    /// This form needs a terminal: tmux answers "open terminal failed: not
    /// a terminal" without one. It belongs on the interactive channel only,
    /// which asks for a pseudo terminal first. To make a session from a
    /// reading channel, use create.
    public static func attach(session: String, tmux: String) -> String? {
        guard TmuxSessionName.isValid(session) else { return nil }
        return tmux + " new -A -s " + session
    }

    /// create makes a session and leaves it running, with no terminal of
    /// its own. The detached form is the one that works over a plain
    /// command channel.
    public static func create(session: String, tmux: String) -> String? {
        guard TmuxSessionName.isValid(session) else { return nil }
        return tmux + " new-session -d -s " + session
    }

    public static func listSessions(tmux: String) -> String {
        tmux + " list-sessions -F \"" + listFormat + "\""
    }

    public static func kill(session: String, tmux: String) -> String? {
        guard TmuxSessionName.isValid(session) else { return nil }
        return tmux + " kill-session -t " + session
    }
}

/// TmuxResolver finds the tmux binary of one host. A non-interactive ssh
/// command on macOS has a PATH without /opt/homebrew/bin, so a bare tmux is
/// not found there. The probe tries the plain name, then a login shell,
/// then the paths that tmux normally takes, and the app keeps the answer
/// for that host.
public enum TmuxResolver {
    public static let knownPaths = ["/usr/bin/tmux", "/usr/local/bin/tmux", "/opt/homebrew/bin/tmux"]

    /// probeCommand asks the three questions in one round trip and prints
    /// the first answer it finds.
    public static var probeCommand: String {
        let known = knownPaths.joined(separator: " ")
        return "command -v tmux 2>/dev/null "
            + "|| \"$SHELL\" -lc 'command -v tmux' 2>/dev/null "
            + "|| for p in " + known + "; do [ -x \"$p\" ] && echo \"$p\" && break; done"
    }

    /// path reads the answer. Only an absolute path counts: a bare name
    /// would depend on the PATH again, and an error line is not a path.
    public static func path(fromProbe output: String) -> String? {
        for line in output.components(separatedBy: .newlines) {
            let text = line.trimmingCharacters(in: .whitespaces)
            if text.hasPrefix("/") { return text }
        }
        return nil
    }
}

/// TmuxCache keeps the resolved path for each host, so the app probes once.
public protocol TmuxCache: Sendable {
    func path(forHost host: String) -> String?
    func setPath(_ path: String, forHost host: String)
    func forget(host: String)
}

/// MemoryTmuxCache holds the paths in memory. The tests use it.
public final class MemoryTmuxCache: TmuxCache, @unchecked Sendable {
    private let lock = NSLock()
    private var paths: [String: String] = [:]

    public init() {}

    public func path(forHost host: String) -> String? { lock.withLock { paths[host] } }
    public func setPath(_ path: String, forHost host: String) { lock.withLock { paths[host] = path } }
    public func forget(host: String) { lock.withLock { paths[host] = nil } }
}

/// TmuxSession is one session on the host.
public struct TmuxSession: Sendable, Equatable, Identifiable, Hashable {
    public let name: String
    public let windows: Int
    public let isAttached: Bool
    public let created: Date
    public let lastActivity: Date

    public var id: String { name }

    /// idleSeconds is the time since the last activity. It never goes
    /// below zero, whatever the two clocks say.
    public func idleSeconds(now: Date) -> Double {
        max(0, now.timeIntervalSince(lastActivity))
    }

    /// parse reads the lines that the explicit format produces. A line
    /// that does not hold every field is skipped, never half read.
    public static func parse(_ output: String) -> [TmuxSession] {
        output.components(separatedBy: .newlines).compactMap { line in
            let fields = line.components(separatedBy: "\t")
            guard fields.count == 5 else { return nil }
            let name = fields[0].trimmingCharacters(in: .whitespaces)
            guard !name.isEmpty,
                let windows = Int(fields[1]),
                let attached = Int(fields[2]),
                let created = Double(fields[3]),
                let activity = Double(fields[4])
            else { return nil }
            return TmuxSession(
                name: name,
                windows: windows,
                isAttached: attached > 0,
                created: Date(timeIntervalSince1970: created),
                lastActivity: Date(timeIntervalSince1970: activity))
        }
    }
}

/// TmuxSessionList reads the answer of tmux list-sessions.
public enum TmuxSessionList: Sendable, Equatable {
    case sessions([TmuxSession])
    case failed(String)

    /// read tells the empty state apart from a failure. tmux exits with a
    /// non-zero status and prints "error connecting to ..." or "no server
    /// running on ..." when nobody started a session yet. That is the
    /// normal empty state on a fresh host, not a fault, and showing it as
    /// an error would make every first connection look broken.
    public static func read(stdout: String, stderr: String, exitStatus: Int32) -> TmuxSessionList {
        if exitStatus == 0 { return .sessions(TmuxSession.parse(stdout)) }
        let message = stderr.trimmingCharacters(in: .whitespacesAndNewlines)
        if message.contains("error connecting to") || message.contains("no server running") {
            return .sessions([])
        }
        if message.isEmpty { return .failed("tmux ended with status \(exitStatus)") }
        return .failed(message)
    }
}
