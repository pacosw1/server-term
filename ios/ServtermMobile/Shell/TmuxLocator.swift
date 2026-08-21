import Foundation
import ServtermSSH

/// DefaultsTmuxCache keeps the resolved tmux path for each host, so the app
/// probes a host once and not on every screen.
struct DefaultsTmuxCache: TmuxCache {
    private let prefix = "servterm.tmux.path."

    init() {}

    func path(forHost host: String) -> String? {
        UserDefaults.standard.string(forKey: prefix + host)
    }

    func setPath(_ path: String, forHost host: String) {
        UserDefaults.standard.set(path, forKey: prefix + host)
    }

    func forget(host: String) {
        UserDefaults.standard.removeObject(forKey: prefix + host)
    }
}

/// TmuxLocator finds tmux on a host and remembers it. It tries the plain
/// name, then a login shell, then the known paths, in one round trip.
struct TmuxLocator: Sendable {
    private let runner: any SSHRunning
    private let cache: any TmuxCache

    init(runner: any SSHRunning, cache: any TmuxCache = DefaultsTmuxCache()) {
        self.runner = runner
        self.cache = cache
    }

    /// locate returns the absolute path of tmux, or nil when the host has
    /// none. A nil answer is not a failure: the shell still works, without
    /// a session that survives.
    func locate(_ request: SSHRequest) async throws -> String? {
        if let known = cache.path(forHost: request.host) { return known }
        let result = try await runner.run(request, command: TmuxResolver.probeCommand)
        guard let path = TmuxResolver.path(fromProbe: result.stdout) else { return nil }
        cache.setPath(path, forHost: request.host)
        return path
    }
}
