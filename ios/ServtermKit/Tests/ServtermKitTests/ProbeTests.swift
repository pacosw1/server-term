import Foundation
import Testing
@testable import ServtermKit

@Suite("Prober")
struct ProbeTests {
    @Test("a host that answers on both ports is reachable twice")
    func bothPorts() async throws {
        let client = FakeHTTPClient(status: 200, body: FixtureJSON.agentStatus)
        let results = await Prober(api: ServtermAPI(client: client)).probe(host: "100.0.0.1")
        #expect(results.count == 2)
        #expect(results.allSatisfy(\.reachable))
        #expect(results.map(\.port) == [7843, 7844])
        #expect(results[0].kind == .agent)
        #expect(results[1].kind == .orchestrator)
    }

    @Test("a port that answers 401 is reachable, because the service is there")
    func unauthorizedIsReachable() async throws {
        let client = FakeHTTPClient(status: 401, body: "unauthorized")
        let results = await Prober(api: ServtermAPI(client: client)).probe(host: "100.0.0.1")
        #expect(results.allSatisfy(\.reachable))
        #expect(results[0].detail == "the service answers, but the token is missing or wrong")
    }

    @Test("a transport failure is not reachable")
    func transportFailure() async throws {
        let failure = URLError(.cannotConnectToHost)
        let client = FakeHTTPClient(status: 0, body: "", failure: failure)
        let results = await Prober(api: ServtermAPI(client: client)).probe(host: "100.0.0.1")
        #expect(results.allSatisfy { !$0.reachable })
        #expect(results[0].detail.isEmpty == false)
    }

    @Test("the prober trims a pasted URL down to a host")
    func hostCleanup() {
        #expect(Prober.cleanHost(" http://100.93.34.43:7843/v1/status ") == "100.93.34.43")
        #expect(Prober.cleanHost("mac-studio.tail1234.ts.net") == "mac-studio.tail1234.ts.net")
        #expect(Prober.cleanHost("") == nil)
        #expect(Prober.cleanHost("   ") == nil)
    }
}
