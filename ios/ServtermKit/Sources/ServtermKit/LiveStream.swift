import Foundation

/// LiveEvent is everything that one live connection can tell the app.
public enum LiveEvent: Sendable, Equatable {
    case connected
    case sample(Sample)
    /// badFrame carries a frame that the app cannot read. The stream keeps
    /// running, because one bad frame is not a broken connection.
    case badFrame(String)
    /// disconnected carries the reason. A reconnect follows after a pause.
    case disconnected(String)
}

/// WebSocketChannel is one socket. The protocol keeps the real network out
/// of the tests.
public protocol WebSocketChannel: Sendable {
    func open(_ request: URLRequest) async throws
    func receive() async throws -> Data
    func close()
}

/// WebSocketFactory makes one channel for each connection attempt. A socket
/// cannot be opened twice, so every attempt needs a new one.
public protocol WebSocketFactory: Sendable {
    func make() -> any WebSocketChannel
}

/// Backoff is the pause between two connection attempts. It grows and then
/// stops at the cap, so a phone never runs a tight retry loop.
public struct Backoff: Sendable {
    public let first: Double
    public let factor: Double
    public let cap: Double

    public init(first: Double = 0.5, factor: Double = 2, cap: Double = 15) {
        self.first = first
        self.factor = factor
        self.cap = cap
    }

    public func delay(attempt: Int) -> Double {
        let steps = max(0, attempt - 1)
        return min(cap, first * pow(factor, Double(steps)))
    }
}

/// LiveStream reads the agent stream endpoint. The agent sends the newest
/// sample at once and then about one sample for each second. Every frame
/// carries the same WireSample shape as the history page, so the stream
/// reuses the history decoder.
public struct LiveStream: Sendable {
    private let factory: any WebSocketFactory
    private let backoff: Backoff
    private let sleep: @Sendable (Double) async -> Void

    public init(
        factory: any WebSocketFactory = URLSessionWebSocketFactory(),
        backoff: Backoff = Backoff(),
        sleep: @escaping @Sendable (Double) async -> Void = { seconds in
            try? await Task.sleep(for: .seconds(seconds))
        }
    ) {
        self.factory = factory
        self.backoff = backoff
        self.sleep = sleep
    }

    /// streamURL turns an agent URL into its stream URL. The agent takes
    /// the token in the Authorization header, never in the query, so no
    /// token ever reaches a URL.
    public static func streamURL(baseURL: String) throws -> URL {
        var text = baseURL.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { throw ServtermError.badURL }
        while text.hasSuffix("/") { text.removeLast() }
        guard var components = URLComponents(string: text), let scheme = components.scheme else {
            throw ServtermError.badURL
        }
        switch scheme {
        case "http", "ws": components.scheme = "ws"
        case "https", "wss": components.scheme = "wss"
        default: throw ServtermError.badURL
        }
        components.path += "/v1/stream"
        guard let url = components.url, url.host != nil else { throw ServtermError.badURL }
        return url
    }

    /// events opens the socket and keeps it open. It reconnects after a
    /// failure, with a growing pause. The caller stops the whole loop by
    /// cancelling its task or by leaving the loop.
    public func events(baseURL: String, token: String) -> AsyncStream<LiveEvent> {
        AsyncStream { continuation in
            let task = Task {
                var attempt = 0
                while !Task.isCancelled {
                    let channel = factory.make()
                    do {
                        let url = try Self.streamURL(baseURL: baseURL)
                        var request = URLRequest(url: url)
                        request.setValue("Bearer " + token, forHTTPHeaderField: "Authorization")
                        try await channel.open(request)
                        attempt = 0
                        continuation.yield(.connected)
                        while !Task.isCancelled {
                            let data = try await channel.receive()
                            if let wire = try? JSONDecoding.agent.decode(WireSample.self, from: data) {
                                continuation.yield(.sample(wire.sample))
                            } else {
                                continuation.yield(.badFrame("the agent sent a frame that the app cannot read"))
                            }
                        }
                    } catch {
                        continuation.yield(.disconnected(Self.reason(error)))
                    }
                    channel.close()
                    if Task.isCancelled { break }
                    attempt += 1
                    await sleep(backoff.delay(attempt: attempt))
                }
                continuation.finish()
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    private static func reason(_ error: any Error) -> String {
        if let known = error as? ServtermError { return known.message }
        return error.localizedDescription
    }
}

/// URLSessionWebSocketFactory makes the real socket.
public struct URLSessionWebSocketFactory: WebSocketFactory {
    private let timeout: TimeInterval

    public init(timeout: TimeInterval = 10) {
        self.timeout = timeout
    }

    public func make() -> any WebSocketChannel {
        URLSessionWebSocketChannel(timeout: timeout)
    }
}

/// URLSessionWebSocketChannel wraps one URLSessionWebSocketTask. The class
/// is not Sendable by itself, so the wrapper guards the task with a lock.
final class URLSessionWebSocketChannel: WebSocketChannel, @unchecked Sendable {
    private let lock = NSLock()
    private var task: URLSessionWebSocketTask?
    private let session: URLSession

    init(timeout: TimeInterval) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.waitsForConnectivity = false
        session = URLSession(configuration: configuration)
    }

    func open(_ request: URLRequest) async throws {
        let task = session.webSocketTask(with: request)
        lock.withLock { self.task = task }
        task.resume()
    }

    func receive() async throws -> Data {
        guard let task = lock.withLock({ self.task }) else {
            throw ServtermError.transport("the socket is closed")
        }
        switch try await task.receive() {
        case .string(let text):
            return Data(text.utf8)
        case .data(let data):
            return data
        @unknown default:
            throw ServtermError.transport("the agent sent an unknown frame")
        }
    }

    func close() {
        let task = lock.withLock { () -> URLSessionWebSocketTask? in
            let current = self.task
            self.task = nil
            return current
        }
        task?.cancel(with: .goingAway, reason: nil)
        session.invalidateAndCancel()
    }
}
