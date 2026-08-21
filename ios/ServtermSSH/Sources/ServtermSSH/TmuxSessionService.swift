import Foundation

/// TmuxSessionService is the whole session flow of one server: find tmux,
/// list the sessions, make one, kill one, and build the plan that the shell
/// screen attaches with.
///
/// It lives in this package on purpose. The app screens are thin callers of
/// it, and a harness on a desktop can drive the SAME functions against a
/// real host. A flow that is only assembled inside a SwiftUI model cannot
/// be run anywhere else, and then nobody can prove it works before a person
/// tries it on a phone.
public struct TmuxSessionService: Sendable {
    /// Listing is the answer of a list, with the empty state told apart
    /// from a failure.
    public enum Listing: Sendable, Equatable {
        case sessions([TmuxSession])
        case failed(String)
    }

    private let runner: any SSHRunning
    private let locator: TmuxLocating

    public init(runner: any SSHRunning, locator: any TmuxLocating) {
        self.runner = runner
        self.locator = locator
    }

    /// resolve finds the tmux binary of a host, or nil when it has none.
    public func resolve(_ request: SSHRequest) async throws -> String? {
        try await locator.locate(request)
    }

    /// list reads the sessions. A host with no tmux server yet answers with
    /// a non-zero status and a plain message, and that is the empty state.
    public func list(_ request: SSHRequest) async -> Listing {
        do {
            guard let tmux = try await resolve(request) else {
                return .failed("The app found no tmux on this host, so it keeps no session.")
            }
            let result = try await runner.run(request, command: TmuxCommand.listSessions(tmux: tmux))
            switch TmuxSessionList.read(
                stdout: result.stdout, stderr: result.stderr, exitStatus: result.exitStatus)
            {
            case .sessions(let sessions):
                return .sessions(sessions.sorted { $0.lastActivity > $1.lastActivity })
            case .failed(let reason):
                return .failed(reason)
            }
        } catch {
            return .failed(Self.reason(error))
        }
    }

    /// create makes a session with the detached form, which needs no
    /// terminal. The attach form would answer "open terminal failed: not a
    /// terminal" on this channel.
    public func create(name: String, request: SSHRequest) async -> String? {
        guard TmuxSessionName.isValid(name) else { return TmuxSessionName.rule }
        return await perform(request: request) { TmuxCommand.create(session: name, tmux: $0) }
    }

    /// kill ends one session. It is the only command that destroys
    /// anything.
    public func kill(name: String, request: SSHRequest) async -> String? {
        await perform(request: request) { TmuxCommand.kill(session: name, tmux: $0) }
    }

    /// plan builds what the shell screen attaches with. A host with no tmux
    /// still gets a shell, and the note says why it does not persist.
    public func plan(session: String, request: SSHRequest) async -> SessionPlan {
        do {
            guard let tmux = try await resolve(request) else {
                return SessionPlan.plainShell(reason: "The app found no tmux on this host")
            }
            return SessionPlan.attach(session: session, tmux: tmux)
                ?? SessionPlan.plainShell(reason: "The app cannot use that session name")
        } catch {
            return SessionPlan.plainShell(reason: "The app could not ask this host about tmux")
        }
    }

    /// perform runs one command and returns nil on success, or the reason
    /// it failed.
    private func perform(
        request: SSHRequest, command build: (String) -> String?
    ) async -> String? {
        do {
            guard let tmux = try await resolve(request) else {
                return "The app found no tmux on this host."
            }
            guard let command = build(tmux) else { return TmuxSessionName.rule }
            let result = try await runner.run(request, command: command)
            guard result.exitStatus != 0 else { return nil }
            let message = result.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
            return message.isEmpty ? "the command ended with status \(result.exitStatus)" : message
        } catch {
            return Self.reason(error)
        }
    }

    static func reason(_ error: any Error) -> String {
        if let error = error as? SSHClientError { return error.message }
        return error.localizedDescription
    }
}

/// TmuxLocating finds tmux on a host. The app caches the answer; a test
/// uses a double.
public protocol TmuxLocating: Sendable {
    func locate(_ request: SSHRequest) async throws -> String?
}

/// TmuxProbeLocator asks the host once and remembers the answer.
public struct TmuxProbeLocator: TmuxLocating {
    private let runner: any SSHRunning
    private let cache: any TmuxCache

    public init(runner: any SSHRunning, cache: any TmuxCache = MemoryTmuxCache()) {
        self.runner = runner
        self.cache = cache
    }

    public func locate(_ request: SSHRequest) async throws -> String? {
        if let known = cache.path(forHost: request.host) { return known }
        let result = try await runner.run(request, command: TmuxResolver.probeCommand)
        guard let path = TmuxResolver.path(fromProbe: result.stdout) else { return nil }
        cache.setPath(path, forHost: request.host)
        return path
    }
}
