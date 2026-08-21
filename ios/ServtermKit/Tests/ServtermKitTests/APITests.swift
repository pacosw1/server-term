import Foundation
import Testing
@testable import ServtermKit

@Suite("ServtermAPI")
struct APITests {
    @Test("the latest sample request asks the agent for one recent minute")
    func historyRequest() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.history)
        let api = ServtermAPI(client: client)
        _ = try await api.latestSample(baseURL: "http://100.0.0.1:7843/", token: "secret")
        let request = try #require(await client.requests.first)
        #expect(request.url?.absoluteString == "http://100.0.0.1:7843/v1/history?minutes=1&limit=8")
        #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer secret")
        #expect(request.httpMethod == "GET")
    }

    @Test("the latest sample is the newest one in the page")
    func newestSample() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.history)
        let api = ServtermAPI(client: client)
        let sample = try await api.latestSample(baseURL: "http://100.0.0.1:7843", token: "secret")
        #expect(abs(sample.cpuPercent - 1.5) < 0.001)
    }

    @Test("an empty history is an error, not an empty reading")
    func emptyHistory() async throws {
        let client = FakeHTTPClient(status: 200, body: "[]")
        let api = ServtermAPI(client: client)
        await #expect(throws: ServtermError.noData) {
            _ = try await api.latestSample(baseURL: "http://100.0.0.1:7843", token: "secret")
        }
    }

    @Test("a bad token reports unauthorized")
    func unauthorized() async throws {
        let client = FakeHTTPClient(status: 401, body: "unauthorized")
        let api = ServtermAPI(client: client)
        await #expect(throws: ServtermError.unauthorized) {
            _ = try await api.latestSample(baseURL: "http://100.0.0.1:7843", token: "wrong")
        }
    }

    @Test("a server fault reports the status code")
    func serverFault() async throws {
        let client = FakeHTTPClient(status: 500, body: "boom")
        let api = ServtermAPI(client: client)
        await #expect(throws: ServtermError.http(500)) {
            _ = try await api.latestSample(baseURL: "http://100.0.0.1:7843", token: "t")
        }
    }

    @Test("damaged JSON reports a decode error, not a healthy zero")
    func damagedJSON() async throws {
        let client = FakeHTTPClient(status: 200, body: "{not json")
        let api = ServtermAPI(client: client)
        var failed = false
        do {
            _ = try await api.latestSample(baseURL: "http://100.0.0.1:7843", token: "t")
        } catch let error as ServtermError {
            if case .decoding = error { failed = true }
        }
        #expect(failed)
    }

    @Test("a bad address reports a bad URL")
    func badURL() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.history)
        let api = ServtermAPI(client: client)
        await #expect(throws: ServtermError.badURL) {
            _ = try await api.latestSample(baseURL: "", token: "t")
        }
    }

    @Test("the orchestrator request uses the status path")
    func orchestratorRequest() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.orchestratorStatus)
        let api = ServtermAPI(client: client)
        let snapshot = try await api.orchestrator(endpoint: "http://100.0.0.1:7844/", token: "k")
        #expect(snapshot.mode == "fast")
        let request = try #require(await client.requests.first)
        #expect(request.url?.absoluteString == "http://100.0.0.1:7844/api/status")
        #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer k")
    }

    @Test("the agent status request needs no token")
    func statusRequest() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.agentStatus)
        let api = ServtermAPI(client: client)
        let status = try await api.agentStatus(baseURL: "http://100.0.0.1:7843")
        #expect(status.nodeID == "node-a")
        let request = try #require(await client.requests.first)
        #expect(request.url?.absoluteString == "http://100.0.0.1:7843/v1/status")
        #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
    }

    @Test("the history request asks for the window that the chart needs")
    func historyWindowRequest() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.history)
        let api = ServtermAPI(client: client)
        let samples = try await api.history(baseURL: "http://100.0.0.1:7843", token: "t", minutes: 10)
        #expect(samples.count == 2)
        #expect(samples[0].at < samples[1].at)
        let request = try #require(await client.requests.first)
        #expect(request.url?.absoluteString == "http://100.0.0.1:7843/v1/history?minutes=10&limit=200")
    }

    @Test("an empty history is an empty chart, not an error")
    func emptyHistoryIsNotAnError() async throws {
        let client = FakeHTTPClient(status: 200, body: "[]")
        let api = ServtermAPI(client: client)
        let samples = try await api.history(baseURL: "http://100.0.0.1:7843", token: "t", minutes: 10)
        #expect(samples.isEmpty)
    }
}
