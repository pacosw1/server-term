import Foundation
import Testing
@testable import ServtermKit

/// FakeChannel replays a script of frames. It never opens a real socket.
private final class FakeChannel: WebSocketChannel, @unchecked Sendable {
    enum Step {
        case frame(String)
        case failure(String)
        case openFailure(String)
    }

    private let lock = NSLock()
    private var steps: [Step]
    private(set) var closed = 0

    init(_ steps: [Step]) { self.steps = steps }

    func open(_ request: URLRequest) async throws {
        let next = lock.withLock { steps.first }
        if case .openFailure(let message) = next {
            _ = lock.withLock { steps.removeFirst() }
            throw ServtermError.transport(message)
        }
    }

    func receive() async throws -> Data {
        let step = lock.withLock { steps.isEmpty ? nil : steps.removeFirst() }
        switch step {
        case .frame(let text):
            return Data(text.utf8)
        case .failure(let message):
            throw ServtermError.transport(message)
        default:
            throw ServtermError.transport("the script is empty")
        }
    }

    func close() {
        lock.withLock { closed += 1 }
    }
}

private final class FakeFactory: WebSocketFactory, @unchecked Sendable {
    private let lock = NSLock()
    private var channels: [FakeChannel]
    private(set) var made: [FakeChannel] = []

    init(_ channels: [FakeChannel]) { self.channels = channels }

    func make() -> any WebSocketChannel {
        lock.withLock {
            let channel = channels.isEmpty ? FakeChannel([.failure("no more")]) : channels.removeFirst()
            made.append(channel)
            return channel
        }
    }
}

private let frame = """
{"version":1,"node_id":"node-a","sample":{"At":"2026-08-21T05:20:53Z","Online":true,
 "Hostname":"node-a","CPUPercent":12.5,"MemTotal":1000,"MemAvailable":400}}
"""

private func collect(
    _ stream: LiveStream, count: Int, baseURL: String = "http://10.0.0.1:7843"
) async -> [LiveEvent] {
    var events: [LiveEvent] = []
    for await event in stream.events(baseURL: baseURL, token: "t") {
        events.append(event)
        if events.count >= count { break }
    }
    return events
}

@Suite("LiveStream")
struct LiveStreamTests {
    private func stream(_ factory: FakeFactory) -> LiveStream {
        LiveStream(
            factory: factory,
            backoff: Backoff(first: 0.5, factor: 2, cap: 15),
            sleep: { _ in })
    }

    @Test("the stream URL follows the scheme of the agent URL")
    func streamURL() throws {
        #expect(try LiveStream.streamURL(baseURL: "http://10.0.0.1:7843").absoluteString
            == "ws://10.0.0.1:7843/v1/stream")
        #expect(try LiveStream.streamURL(baseURL: "https://host/agent/").absoluteString
            == "wss://host/agent/v1/stream")
        #expect(throws: ServtermError.badURL) {
            _ = try LiveStream.streamURL(baseURL: "")
        }
    }

    @Test("a frame arrives as a sample")
    func publishesSample() async {
        let factory = FakeFactory([FakeChannel([.frame(frame), .failure("closed")])])
        let events = await collect(stream(factory), count: 2)
        #expect(events[0] == .connected)
        guard case .sample(let sample) = events[1] else {
            Issue.record("the stream did not publish a sample")
            return
        }
        #expect(sample.hostname == "node-a")
        #expect(abs(sample.cpuPercent - 12.5) < 0.001)
    }

    @Test("a close is reported to the screen")
    func reportsClose() async {
        let factory = FakeFactory([FakeChannel([.frame(frame), .failure("the socket closed")])])
        let events = await collect(stream(factory), count: 3)
        guard case .disconnected(let message) = events[2] else {
            Issue.record("the stream did not report the close")
            return
        }
        #expect(message.contains("closed"))
    }

    @Test("a bad frame does not kill the stream")
    func survivesBadFrame() async {
        let channel = FakeChannel([.frame("{not json"), .frame(frame), .failure("end")])
        let events = await collect(stream(FakeFactory([channel])), count: 3)
        guard case .badFrame = events[1] else {
            Issue.record("the stream did not report the bad frame")
            return
        }
        guard case .sample = events[2] else {
            Issue.record("the stream stopped after the bad frame")
            return
        }
    }

    @Test("the stream opens a new socket after a failure")
    func reconnects() async {
        let first = FakeChannel([.failure("dropped")])
        let second = FakeChannel([.frame(frame), .failure("end")])
        let factory = FakeFactory([first, second])
        let events = await collect(stream(factory), count: 4)
        #expect(events.count == 4)
        guard case .sample = events[3] else {
            Issue.record("the stream did not read from the second socket")
            return
        }
        // The producer may already be opening the next socket when the
        // reader stops, so the count is at least two, not exactly two.
        #expect(factory.made.count >= 2)
        #expect(first.closed >= 1)
    }

    @Test("a socket that cannot open is reported, not retried without a pause")
    func openFailure() async {
        let factory = FakeFactory([FakeChannel([.openFailure("no route")])])
        let events = await collect(stream(factory), count: 1)
        guard case .disconnected(let message) = events[0] else {
            Issue.record("the stream did not report the open failure")
            return
        }
        #expect(message.contains("no route"))
    }

    @Test("the backoff grows and then stops at the cap")
    func backoff() {
        let backoff = Backoff(first: 0.5, factor: 2, cap: 8)
        #expect(backoff.delay(attempt: 1) == 0.5)
        #expect(backoff.delay(attempt: 2) == 1)
        #expect(backoff.delay(attempt: 3) == 2)
        #expect(backoff.delay(attempt: 4) == 4)
        #expect(backoff.delay(attempt: 5) == 8)
        #expect(backoff.delay(attempt: 9) == 8)
        #expect(backoff.delay(attempt: 0) == 0.5)
    }
}
