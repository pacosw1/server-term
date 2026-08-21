import Foundation
import Observation
import ServtermKit

/// Reading holds one fetch result. It keeps the value, the time of the
/// value, and the last error apart, so a screen never shows an old reading
/// as a fresh one, and never shows an error as a healthy zero.
struct Reading<Value: Sendable>: Sendable {
    var value: Value?
    var fetchedAt: Date?
    var error: String?
    var isLoading: Bool = false

    var hasValue: Bool { value != nil }
}

/// AppModel keeps the app setup and the last reading of every service. It
/// saves the setup in UserDefaults and every token in the Keychain.
@MainActor
@Observable
final class AppModel {
    private(set) var config: AppConfig
    var servers: [UUID: Reading<Sample>] = [:]
    var orchestrator = Reading<OrchestratorSnapshot>()

    private let api: ServtermAPI
    private let tokens: any TokenStore
    private let defaults: UserDefaults
    private let configKey = "servterm.config.v1"

    init(
        api: ServtermAPI = ServtermAPI(),
        tokens: any TokenStore = KeychainTokenStore(),
        defaults: UserDefaults = .standard
    ) {
        self.api = api
        self.tokens = tokens
        self.defaults = defaults
        if let data = defaults.data(forKey: configKey),
            let saved = try? JSONDecoder().decode(AppConfig.self, from: data)
        {
            config = saved
        } else {
            config = AppConfig()
        }
    }

    // MARK: - setup

    func save(_ newConfig: AppConfig) {
        config = newConfig
        if let data = try? JSONEncoder().encode(newConfig) {
            defaults.set(data, forKey: configKey)
        }
    }

    func upsert(server: ServerEntry, token: String?) {
        var next = config
        next.upsert(server)
        save(next)
        if let token { tokens.setToken(token, for: server.id.uuidString) }
    }

    func remove(server: ServerEntry) {
        var next = config
        next.remove(serverID: server.id)
        save(next)
        tokens.removeToken(for: server.id.uuidString)
        servers[server.id] = nil
    }

    func setOrchestrator(_ entry: OrchestratorEntry?, token: String?) {
        var next = config
        if let old = next.orchestrator, old.id != entry?.id {
            tokens.removeToken(for: old.id.uuidString)
        }
        next.orchestrator = entry
        save(next)
        if let entry, let token { tokens.setToken(token, for: entry.id.uuidString) }
        if entry == nil { orchestrator = Reading() }
    }

    func token(for id: UUID) -> String { tokens.token(for: id.uuidString) ?? "" }

    /// merge adds the imported servers. It keeps the servers that the user
    /// already set up, and it never brings a token with it.
    func merge(_ imported: ImportedConfig) {
        var next = config
        for server in imported.servers where !next.servers.contains(where: { $0.agentURL == server.agentURL }) {
            next.servers.append(server)
        }
        if next.orchestrator == nil { next.orchestrator = imported.orchestrator }
        save(next)
    }

    // MARK: - reading

    func refreshAllServers() async {
        await withTaskGroup(of: Void.self) { group in
            for server in config.servers {
                group.addTask { await self.refresh(server: server) }
            }
        }
    }

    func refresh(server: ServerEntry) async {
        let id = server.id
        var reading = servers[id] ?? Reading<Sample>()
        reading.isLoading = true
        servers[id] = reading
        let token = token(for: id)
        do {
            let sample = try await api.latestSample(baseURL: server.agentURL, token: token)
            reading.value = sample
            reading.fetchedAt = Date()
            reading.error = sample.online ? nil : agentReported(sample)
        } catch let error as ServtermError {
            reading.error = error.message
        } catch {
            reading.error = error.localizedDescription
        }
        reading.isLoading = false
        servers[id] = reading
    }

    func refreshOrchestrator() async {
        guard let entry = config.orchestrator else {
            orchestrator = Reading()
            return
        }
        orchestrator.isLoading = true
        let token = token(for: entry.id)
        do {
            let snapshot = try await api.orchestrator(endpoint: entry.endpoint, token: token)
            orchestrator.value = snapshot
            orchestrator.fetchedAt = Date()
            orchestrator.error = snapshot.error.isEmpty ? nil : snapshot.error
        } catch let error as ServtermError {
            orchestrator.error = error.message
        } catch {
            orchestrator.error = error.localizedDescription
        }
        orchestrator.isLoading = false
    }

    /// probe tests the standard servterm ports on one host.
    func probe(host: String) async -> [ProbeResult] {
        await Prober(api: api).probe(host: host)
    }

    private func agentReported(_ sample: Sample) -> String {
        sample.error.isEmpty ? "the agent reports that the server is offline" : sample.error
    }
}
