import Foundation

/// ServtermError is every failure that the app can meet. Each case carries
/// a short message for the error banner.
public enum ServtermError: Error, Equatable, Sendable {
    case badURL
    case unauthorized
    case http(Int)
    case transport(String)
    case decoding(String)
    case noData
    case importFailed(String)

    public var message: String {
        switch self {
        case .badURL:
            return "the address is not valid"
        case .unauthorized:
            return "the token is missing or wrong"
        case .http(let code):
            return "the server answered with status \(code)"
        case .transport(let detail):
            return "no answer from the server: \(detail)"
        case .decoding(let detail):
            return "the answer is not readable: \(detail)"
        case .noData:
            return "the server has no recent reading"
        case .importFailed(let detail):
            return "the import failed: \(detail)"
        }
    }
}

/// HTTPClient hides the network behind one function, so a test can pass a
/// double instead of a real connection.
public protocol HTTPClient: Sendable {
    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse)
}

/// URLSessionHTTPClient is the real network. The endpoints use plain HTTP
/// on the tailnet, so the app Info.plist allows an insecure load.
public struct URLSessionHTTPClient: HTTPClient {
    private let session: URLSession

    public init(timeout: TimeInterval = 5) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.waitsForConnectivity = false
        session = URLSession(configuration: configuration)
    }

    public func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw ServtermError.transport("the answer is not HTTP")
        }
        return (data, http)
    }
}

/// ServtermAPI reads the servterm services. It performs no write.
public struct ServtermAPI: Sendable {
    private let client: any HTTPClient

    public init(client: any HTTPClient = URLSessionHTTPClient()) {
        self.client = client
    }

    /// agentStatus reads the liveness endpoint. It needs no token.
    public func agentStatus(baseURL: String) async throws -> AgentStatus {
        let request = try Self.request(base: baseURL, path: "/v1/status", query: nil, token: nil)
        let data = try await perform(request)
        return try decode(AgentStatus.self, from: data, with: JSONDecoding.agent)
    }

    /// latestSample reads the newest stored reading. The agent stores one
    /// reading each second, so a short window always holds a fresh one.
    public func latestSample(baseURL: String, token: String) async throws -> Sample {
        let request = try Self.request(
            base: baseURL, path: "/v1/history", query: "minutes=1&limit=8", token: token)
        let data = try await perform(request)
        let page = try decode([WireSample].self, from: data, with: JSONDecoding.agent)
        guard let newest = page.map(\.sample).max(by: { $0.at < $1.at }) else {
            throw ServtermError.noData
        }
        return newest
    }

    /// orchestrator reads the agent orchestrator snapshot.
    public func orchestrator(endpoint: String, token: String) async throws -> OrchestratorSnapshot {
        let request = try Self.request(base: endpoint, path: "/api/status", query: nil, token: token)
        let data = try await perform(request)
        return try decode(OrchestratorSnapshot.self, from: data, with: JSONDecoding.orchestrator)
    }

    /// reach reports whether a service answers at all. A 401 answer still
    /// proves that the service runs on that port.
    public func reach(base: String, path: String) async -> ProbeOutcome {
        guard let request = try? Self.request(base: base, path: path, query: nil, token: nil) else {
            return .init(reachable: false, detail: ServtermError.badURL.message)
        }
        do {
            let (_, response) = try await client.send(request)
            if response.statusCode == 401 {
                return .init(
                    reachable: true,
                    detail: "the service answers, but the token is missing or wrong")
            }
            if (200..<300).contains(response.statusCode) {
                return .init(reachable: true, detail: "the service answers")
            }
            return .init(reachable: true, detail: "the service answers with status \(response.statusCode)")
        } catch {
            return .init(reachable: false, detail: Self.transportDetail(error))
        }
    }

    // MARK: - private

    private func perform(_ request: URLRequest) async throws -> Data {
        let data: Data
        let response: HTTPURLResponse
        do {
            (data, response) = try await client.send(request)
        } catch let error as ServtermError {
            throw error
        } catch {
            throw ServtermError.transport(Self.transportDetail(error))
        }
        if response.statusCode == 401 || response.statusCode == 403 {
            throw ServtermError.unauthorized
        }
        guard (200..<300).contains(response.statusCode) else {
            throw ServtermError.http(response.statusCode)
        }
        return data
    }

    private func decode<T: Decodable>(_ type: T.Type, from data: Data, with decoder: JSONDecoder) throws -> T {
        do {
            return try decoder.decode(type, from: data)
        } catch {
            throw ServtermError.decoding(String(describing: error))
        }
    }

    static func transportDetail(_ error: any Error) -> String {
        (error as? URLError)?.localizedDescription ?? error.localizedDescription
    }

    static func request(base: String, path: String, query: String?, token: String?) throws -> URLRequest {
        let trimmed = base.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { throw ServtermError.badURL }
        var text = trimmed
        while text.hasSuffix("/") { text.removeLast() }
        text += path
        if let query, !query.isEmpty { text += "?" + query }
        guard let url = URL(string: text), url.scheme != nil, url.host != nil else {
            throw ServtermError.badURL
        }
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        if let token, !token.isEmpty {
            request.setValue("Bearer " + token, forHTTPHeaderField: "Authorization")
        }
        return request
    }
}
