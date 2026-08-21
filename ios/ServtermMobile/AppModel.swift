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

/// Transport says where a reading comes from. The screen must show this,
/// because a poll every 3 seconds is not the same as a live socket.
enum Transport: Equatable, Sendable {
    case idle
    case live
    /// polling carries the reason why the socket is not in use.
    case polling(String)

    var isLive: Bool { self == .live }
}

/// AppModel keeps the app setup and the last reading of every service. It
/// saves the setup in UserDefaults and every token in the Keychain.
@MainActor
@Observable
final class AppModel {
    private(set) var config: AppConfig
    var servers: [UUID: Reading<Sample>] = [:]
    /// trends holds a short rolling CPU window for each server, so a card
    /// can draw a sparkline without one more request.
    var trends: [UUID: [MetricPoint]] = [:]
    /// histories holds the wider window that a detail chart needs.
    var histories: [UUID: [Sample]] = [:]
    /// previousSamples holds the reading before the current one. A runner
    /// job CPU needs two readings to exist at all.
    private var previousSamples: [UUID: Sample] = [:]
    /// transports says, for each server, whether a socket or a poll feeds
    /// the screen.
    var transports: [UUID: Transport] = [:]
    /// roundTrips holds the time of the last poll request.
    var roundTrips: [UUID: TimeInterval] = [:]

    private let live: LiveStream
    private var liveTasks: [UUID: Task<Void, Never>] = [:]
    private var liveWants: [String: Set<UUID>] = [:]
    private var badFrames: [UUID: Int] = [:]
    private var isSuspended = false
    var orchestrator = Reading<OrchestratorSnapshot>()
    /// bootstrapMessage tells the user what the one-time import file did.
    /// It never holds a token value.
    var bootstrapMessage: String?
    var bootstrapFailed = false
    /// settingsMessage reports a save that did not work, for example a
    /// Keychain that refuses the write.
    var settingsMessage: String?

    private let api: ServtermAPI
    private let tokens: any TokenStore
    private let defaults: UserDefaults
    private let configKey = "servterm.config.v1"

    init(
        api: ServtermAPI = ServtermAPI(),
        tokens: any TokenStore = KeychainTokenStore(),
        defaults: UserDefaults = .standard,
        live: LiveStream = LiveStream()
    ) {
        self.api = api
        self.live = live
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
        settingsMessage = nil
        if let token, !tokens.setToken(token, for: server.id.uuidString) {
            settingsMessage = Self.keychainFailure
        }
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
        settingsMessage = nil
        if let entry, let token, !tokens.setToken(token, for: entry.id.uuidString) {
            settingsMessage = Self.keychainFailure
        }
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

    // MARK: - one-time import

    /// runBootstrapImport looks for the import file in the Documents folder
    /// of the app. The file carries the hosts and the tokens, so the import
    /// deletes it at once. The app never writes a token to a log.
    func runBootstrapImport(files: any FileAccess = SystemFileAccess(), directory: URL? = nil) {
        let folder = directory
            ?? FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
        guard let folder else { return }
        let url = folder.appendingPathComponent(BootstrapImport.fileName)
        switch BootstrapImport.load(from: url, files: files) {
        case .none:
            return
        case .failed(let message):
            bootstrapFailed = true
            bootstrapMessage = message
        case .imported(let result):
            apply(result)
        }
    }

    /// apply merges the imported setup. It keeps the entry that already
    /// holds a host, so a second import makes no copy of it.
    /// keychainFailure is the one message for a token that the app cannot
    /// store. The app must never read a lost token as a saved one.
    static let keychainFailure =
        "The app cannot save the token in the Keychain. The servers stay unauthorized."

    private func apply(_ result: BootstrapImport.Result) {
        var saved = true
        var next = config
        for server in result.config.servers {
            let token = result.tokens[server.id.uuidString]
            if let index = next.servers.firstIndex(where: { $0.agentURL == server.agentURL }) {
                var existing = next.servers[index]
                existing.name = server.name
                existing.location = server.location
                next.servers[index] = existing
                if let token, !tokens.setToken(token, for: existing.id.uuidString) { saved = false }
            } else {
                next.servers.append(server)
                if let token, !tokens.setToken(token, for: server.id.uuidString) { saved = false }
            }
        }
        if let imported = result.config.orchestrator {
            let token = result.tokens[imported.id.uuidString]
            if let existing = next.orchestrator, existing.endpoint == imported.endpoint {
                if let token, !tokens.setToken(token, for: existing.id.uuidString) { saved = false }
            } else {
                next.orchestrator = imported
                if let token, !tokens.setToken(token, for: imported.id.uuidString) { saved = false }
            }
        }
        save(next)
        bootstrapFailed = !saved
        bootstrapMessage = saved
            ? "The app imported \(result.config.servers.count) servers and deleted the import file."
            : Self.keychainFailure
    }

    // MARK: - reading

    /// refreshAllServers reads every server that no socket feeds. A pull to
    /// refresh forces a read of all of them.
    func refreshAllServers(force: Bool = false) async {
        await withTaskGroup(of: Void.self) { group in
            for server in config.servers where force || !isFresh(server.id) {
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
        let started = Date()
        do {
            let sample = try await api.latestSample(baseURL: server.agentURL, token: token)
            roundTrips[id] = Date().timeIntervalSince(started)
            servers[id]?.isLoading = false
            apply(sample: sample, to: id)
            return
        } catch let error as ServtermError {
            reading.error = error.message
        } catch {
            reading.error = error.localizedDescription
        }
        roundTrips[id] = Date().timeIntervalSince(started)
        reading.isLoading = false
        servers[id] = reading
    }

    /// apply stores one fresh reading. The poll and the socket both end
    /// here, so a reading looks the same whatever brought it.
    private func apply(sample: Sample, to id: UUID) {
        var reading = servers[id] ?? Reading<Sample>()
        if let old = reading.value, old.at != sample.at { previousSamples[id] = old }
        trends[id] = MetricSeries.append(
            MetricPoint(at: sample.at, value: sample.cpuPercent, name: "cpu"),
            to: trends[id] ?? [], limit: 60)
        reading.value = sample
        reading.fetchedAt = Date()
        reading.isLoading = false
        reading.error = sample.online ? nil : agentReported(sample)
        servers[id] = reading
    }

    // MARK: - live connections

    /// setLiveWants records which servers one screen shows. The app opens a
    /// socket for the union of every visible screen, and closes the rest.
    func setLiveWants(_ ids: Set<UUID>, for screen: String) {
        liveWants[screen] = ids
        syncLive()
    }

    /// suspendLive closes every socket when the app leaves the screen. A
    /// phone in a pocket must not hold a socket open.
    func suspendLive() {
        isSuspended = true
        syncLive()
    }

    func resumeLive() {
        isSuspended = false
        syncLive()
    }

    /// isFresh says whether the socket delivered a reading a moment ago.
    /// The poll uses it to stay out of the way of a healthy socket.
    func isFresh(_ id: UUID, now: Date = Date(), limit: TimeInterval = 8) -> Bool {
        guard transports[id]?.isLive == true, let at = servers[id]?.fetchedAt else { return false }
        return now.timeIntervalSince(at) < limit
    }

    private var wantedLiveIDs: Set<UUID> {
        guard !isSuspended else { return [] }
        let known = Set(config.servers.map(\.id))
        return liveWants.values.reduce(into: Set<UUID>()) { $0.formUnion($1) }.intersection(known)
    }

    private func syncLive() {
        let wanted = wantedLiveIDs
        for (id, task) in liveTasks where !wanted.contains(id) {
            task.cancel()
            liveTasks[id] = nil
            transports[id] = .idle
        }
        for server in config.servers where wanted.contains(server.id) && liveTasks[server.id] == nil {
            startLive(server)
        }
    }

    private func startLive(_ server: ServerEntry) {
        let id = server.id
        let token = token(for: id)
        let url = server.agentURL
        transports[id] = .polling("the app is opening the live connection")
        liveTasks[id] = Task { [weak self, live] in
            for await event in live.events(baseURL: url, token: token) {
                guard let self else { return }
                await self.handle(event, for: id)
            }
        }
    }

    private func handle(_ event: LiveEvent, for id: UUID) {
        switch event {
        case .connected:
            badFrames[id] = 0
            transports[id] = .live
        case .sample(let sample):
            transports[id] = .live
            apply(sample: sample, to: id)
        case .badFrame(let message):
            badFrames[id, default: 0] += 1
            // One bad frame is noise. Three in a row means the app cannot
            // trust the socket, so it says that and the poll takes over.
            if badFrames[id, default: 0] >= 3 {
                transports[id] = .polling(message)
            }
        case .disconnected(let message):
            transports[id] = .polling(message)
        }
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

    /// loadHistory reads a wider window for the charts of a detail screen.
    /// A failure leaves the old window alone and reports on the reading, so
    /// the chart never shows an empty history as a fact.
    func loadHistory(server: ServerEntry, minutes: Int = 10) async {
        do {
            let samples = try await api.history(
                baseURL: server.agentURL, token: token(for: server.id), minutes: minutes)
            histories[server.id] = samples
        } catch let error as ServtermError {
            servers[server.id, default: Reading<Sample>()].error = error.message
        } catch {
            servers[server.id, default: Reading<Sample>()].error = error.localizedDescription
        }
    }

    /// cpuTrend is the rolling window for a card. It falls back to the
    /// wider window when the app already holds one.
    func cpuTrend(for id: UUID) -> [MetricPoint] {
        let rolling = trends[id] ?? []
        guard rolling.count < 2, let samples = histories[id] else { return rolling }
        return MetricSeries.cpu(from: samples)
    }

    /// jobCPU derives the CPU of one runner job from two readings.
    func jobCPU(pid: Int, serverID: UUID) -> Double? {
        guard let current = servers[serverID]?.value else { return nil }
        return RunnerMath.jobCPU(pid: pid, previous: previousSamples[serverID], current: current)
    }

    /// runnerServers lists only the servers that report a runner.
    var runnerServers: [ServerEntry] {
        config.servers.filter { servers[$0.id]?.value?.hasRunners == true }
    }

    /// probe tests the standard servterm ports on one host.
    func probe(host: String) async -> [ProbeResult] {
        await Prober(api: api).probe(host: host)
    }

    private func agentReported(_ sample: Sample) -> String {
        sample.error.isEmpty ? "the agent reports that the server is offline" : sample.error
    }
}
