import Foundation
import Testing
import ServtermKit
@testable import ServtermMobile

/// ScriptedChannel replays frames. No test in this bundle opens a socket.
private final class ScriptedChannel: WebSocketChannel, @unchecked Sendable {
    private let lock = NSLock()
    private var frames: [String]
    private(set) var closed = 0
    private let holdOpen: Bool

    init(frames: [String], holdOpen: Bool = false) {
        self.frames = frames
        self.holdOpen = holdOpen
    }

    func open(_ request: URLRequest) async throws {}

    func receive() async throws -> Data {
        if let next = lock.withLock({ frames.isEmpty ? nil : frames.removeFirst() }) {
            return Data(next.utf8)
        }
        if holdOpen {
            // Wait for the reader to cancel, like a quiet but healthy socket.
            try await Task.sleep(for: .seconds(30))
        }
        throw ServtermError.transport("the socket closed")
    }

    func close() { lock.withLock { closed += 1 } }
}

private final class ScriptedFactory: WebSocketFactory, @unchecked Sendable {
    private let lock = NSLock()
    private var channels: [ScriptedChannel]
    private(set) var made: [ScriptedChannel] = []

    init(_ channels: [ScriptedChannel]) { self.channels = channels }

    func make() -> any WebSocketChannel {
        lock.withLock {
            let channel = channels.isEmpty
                ? ScriptedChannel(frames: [], holdOpen: true) : channels.removeFirst()
            made.append(channel)
            return channel
        }
    }
}

private let liveFrame = """
{"version":1,"node_id":"n1","sample":{"At":"2026-08-21T05:20:53Z","Online":true,
 "Hostname":"n1","CPUPercent":42.5,"MemTotal":1000,"MemAvailable":400}}
"""

@MainActor
@Suite("Live in the app")
struct LiveAppTests {
    private func makeModel(_ factory: ScriptedFactory) -> AppModel {
        AppModel(
            api: ServtermAPI(client: URLSessionHTTPClient()),
            tokens: MemoryTokenStore(),
            defaults: UserDefaults(suiteName: "test." + UUID().uuidString)!,
            live: LiveStream(factory: factory, backoff: Backoff(first: 0.01, factor: 1, cap: 0.01),
                             sleep: { _ in }))
    }

    private func waitFor(_ check: @MainActor () -> Bool, seconds: Double = 2) async {
        let deadline = Date().addingTimeInterval(seconds)
        while Date() < deadline {
            if check() { return }
            try? await Task.sleep(for: .milliseconds(20))
        }
    }

    @Test("a frame from the socket becomes the reading, and the transport says live")
    func liveSample() async {
        let factory = ScriptedFactory([ScriptedChannel(frames: [liveFrame], holdOpen: true)])
        let model = makeModel(factory)
        let server = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843")
        model.upsert(server: server, token: "t")
        model.setLiveWants([server.id], for: "servers")

        await waitFor { model.servers[server.id]?.value != nil }
        #expect(abs((model.servers[server.id]?.value?.cpuPercent ?? 0) - 42.5) < 0.001)
        #expect(model.transports[server.id] == .live)
        #expect(model.isFresh(server.id))
        model.setLiveWants([], for: "servers")
    }

    @Test("a closed socket falls back to the poll, and the screen is told why")
    func fallsBackToPolling() async {
        let factory = ScriptedFactory([ScriptedChannel(frames: [])])
        let model = makeModel(factory)
        let server = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843")
        model.upsert(server: server, token: "t")
        model.setLiveWants([server.id], for: "servers")

        await waitFor {
            if case .polling = model.transports[server.id] { return true }
            return false
        }
        guard case .polling(let reason) = model.transports[server.id] else {
            Issue.record("the app did not fall back to the poll")
            return
        }
        #expect(reason.isEmpty == false)
        #expect(model.isFresh(server.id) == false)
        model.setLiveWants([], for: "servers")
    }

    @Test("a screen that goes away closes its socket")
    func closesOnLeave() async {
        let channel = ScriptedChannel(frames: [liveFrame], holdOpen: true)
        let model = makeModel(ScriptedFactory([channel]))
        let server = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843")
        model.upsert(server: server, token: "t")
        model.setLiveWants([server.id], for: "servers")
        await waitFor { model.transports[server.id] == .live }

        model.setLiveWants([], for: "servers")
        await waitFor { channel.closed > 0 }
        #expect(channel.closed > 0)
        #expect(model.transports[server.id] == .idle)
    }

    @Test("the app closes every socket in the background and opens them again")
    func suspendAndResume() async {
        let first = ScriptedChannel(frames: [liveFrame], holdOpen: true)
        let second = ScriptedChannel(frames: [liveFrame], holdOpen: true)
        let factory = ScriptedFactory([first, second])
        let model = makeModel(factory)
        let server = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843")
        model.upsert(server: server, token: "t")
        model.setLiveWants([server.id], for: "servers")
        await waitFor { model.transports[server.id] == .live }

        model.suspendLive()
        await waitFor { first.closed > 0 }
        #expect(first.closed > 0)

        model.resumeLive()
        await waitFor { factory.made.count >= 2 }
        #expect(factory.made.count >= 2)
        model.setLiveWants([], for: "servers")
    }

    @Test("a fresh live reading is not fresh once it ages")
    func freshnessExpires() async {
        let model = makeModel(ScriptedFactory([ScriptedChannel(frames: [liveFrame], holdOpen: true)]))
        let server = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843")
        model.upsert(server: server, token: "t")
        model.setLiveWants([server.id], for: "servers")
        await waitFor { model.transports[server.id] == .live }

        #expect(model.isFresh(server.id, now: Date()))
        #expect(model.isFresh(server.id, now: Date().addingTimeInterval(60)) == false)
        model.setLiveWants([], for: "servers")
    }
}
