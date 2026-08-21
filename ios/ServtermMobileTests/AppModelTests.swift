import Foundation
import Testing
import ServtermKit
@testable import ServtermMobile

/// StubHTTPClient answers each request from a prepared list. No test in this
/// bundle opens a real connection.
private final class StubHTTPClient: HTTPClient, @unchecked Sendable {
    enum Step {
        case ok(String)
        case status(Int)
        case failure
    }

    private let lock = NSLock()
    private var steps: [Step]

    init(_ steps: [Step]) { self.steps = steps }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        let step: Step = lock.withLock { steps.count > 1 ? steps.removeFirst() : (steps.first ?? .status(500)) }
        switch step {
        case .ok(let body):
            let response = HTTPURLResponse(url: request.url!, statusCode: 200, httpVersion: nil, headerFields: nil)!
            return (Data(body.utf8), response)
        case .status(let code):
            let response = HTTPURLResponse(url: request.url!, statusCode: code, httpVersion: nil, headerFields: nil)!
            return (Data(), response)
        case .failure:
            throw URLError(.cannotConnectToHost)
        }
    }
}

private let historyBody = """
[{"version":1,"node_id":"n1","sample":{"At":"2026-08-21T04:11:03.161Z","Online":true,
  "Hostname":"n1","CPUPercent":12.5,"MemTotal":1000,"MemAvailable":400,
  "Disks":[{"Mount":"/","Device":"d","FSType":"ext4","Total":100,"Used":50}]}}]
"""

@MainActor
@Suite("AppModel")
struct AppModelTests {
    private func makeDefaults() -> UserDefaults {
        let name = "test." + UUID().uuidString
        return UserDefaults(suiteName: name)!
    }

    private func makeServer() -> ServerEntry {
        ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843", location: "Lab")
    }

    @Test("a good answer keeps the reading and no error")
    func success() async {
        let model = AppModel(
            api: ServtermAPI(client: StubHTTPClient([.ok(historyBody)])),
            tokens: MemoryTokenStore(), defaults: makeDefaults())
        let server = makeServer()
        model.upsert(server: server, token: "t")
        await model.refresh(server: server)
        let reading = model.servers[server.id]
        #expect(reading?.error == nil)
        #expect(reading?.value?.cpuPercent == 12.5)
        #expect(reading?.fetchedAt != nil)
    }

    @Test("a failed fetch reports an error and holds no reading")
    func failureIsNotHealthy() async {
        let model = AppModel(
            api: ServtermAPI(client: StubHTTPClient([.failure])),
            tokens: MemoryTokenStore(), defaults: makeDefaults())
        let server = makeServer()
        model.upsert(server: server, token: "t")
        await model.refresh(server: server)
        let reading = model.servers[server.id]
        #expect(reading?.value == nil)
        #expect(reading?.fetchedAt == nil)
        #expect(reading?.error != nil)
    }

    @Test("a wrong token reports the token, not a zero reading")
    func unauthorized() async {
        let model = AppModel(
            api: ServtermAPI(client: StubHTTPClient([.status(401)])),
            tokens: MemoryTokenStore(), defaults: makeDefaults())
        let server = makeServer()
        model.upsert(server: server, token: "bad")
        await model.refresh(server: server)
        #expect(model.servers[server.id]?.error == ServtermError.unauthorized.message)
    }

    @Test("a later failure keeps the old reading and its old time")
    func staleAfterFailure() async {
        let model = AppModel(
            api: ServtermAPI(client: StubHTTPClient([.ok(historyBody), .failure])),
            tokens: MemoryTokenStore(), defaults: makeDefaults())
        let server = makeServer()
        model.upsert(server: server, token: "t")
        await model.refresh(server: server)
        let firstTime = model.servers[server.id]?.fetchedAt
        await model.refresh(server: server)
        let reading = model.servers[server.id]
        #expect(reading?.error != nil)
        #expect(reading?.value != nil)
        #expect(reading?.fetchedAt == firstTime)
    }

    @Test("the setup survives a restart, and the token stays out of it")
    func persistence() {
        let defaults = makeDefaults()
        let tokens = MemoryTokenStore()
        let first = AppModel(api: ServtermAPI(client: StubHTTPClient([.failure])), tokens: tokens, defaults: defaults)
        let server = makeServer()
        first.upsert(server: server, token: "secret")
        let second = AppModel(api: ServtermAPI(client: StubHTTPClient([.failure])), tokens: tokens, defaults: defaults)
        #expect(second.config.servers.map(\.name) == ["one"])
        #expect(second.token(for: server.id) == "secret")
        let saved = defaults.data(forKey: "servterm.config.v1")!
        #expect(String(data: saved, encoding: .utf8)?.contains("secret") == false)
    }

    @Test("removing a server removes its token too")
    func removeToken() {
        let tokens = MemoryTokenStore()
        let model = AppModel(api: ServtermAPI(client: StubHTTPClient([.failure])), tokens: tokens, defaults: makeDefaults())
        let server = makeServer()
        model.upsert(server: server, token: "secret")
        model.remove(server: server)
        #expect(model.config.servers.isEmpty)
        #expect(tokens.token(for: server.id.uuidString) == nil)
    }

    @Test("the import adds a host once, even after two imports")
    func importIsIdempotent() throws {
        let model = AppModel(api: ServtermAPI(client: StubHTTPClient([.failure])), tokens: MemoryTokenStore(), defaults: makeDefaults())
        let text = """
        {"servers":[{"name":"a","agent_url":"http://10.0.0.1:7843"}],
         "widgets":[{"name":"w","type":"orchestrator","endpoint":"http://10.0.0.1:7844"}]}
        """
        let imported = try ConfigImport.parse(text)
        model.merge(imported)
        model.merge(imported)
        #expect(model.config.servers.count == 1)
        #expect(model.config.orchestrator?.endpoint == "http://10.0.0.1:7844")
    }
}
