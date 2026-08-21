import Foundation
import Testing
@testable import ServtermKit

@Suite("Decoding")
struct DecodingTests {
    @Test("the date parser reads every offset and fraction that Go writes")
    func dates() throws {
        let nanoseconds = try #require(ServtermDate.parse("2026-08-21T06:10:56.998458518+02:00"))
        #expect(abs(nanoseconds.timeIntervalSince1970 - 1787285456.998) < 0.002)
        let milliseconds = try #require(ServtermDate.parse("2026-08-21T04:11:03.161Z"))
        #expect(abs(milliseconds.timeIntervalSince1970 - 1787285463.161) < 0.002)
        let seconds = try #require(ServtermDate.parse("2026-08-21T04:11:03Z"))
        #expect(abs(seconds.timeIntervalSince1970 - 1787285463) < 0.002)
        #expect(ServtermDate.parse("not a date") == nil)
    }

    @Test("agent status decodes")
    func agentStatus() throws {
        let status = try JSONDecoding.agent.decode(AgentStatus.self, from: Data(FixtureJSON.agentStatus.utf8))
        #expect(status.service == "servterm-agent")
        #expect(status.nodeID == "node-a")
        #expect(status.version == 1)
    }

    @Test("a history page decodes with the Go field names")
    func sample() throws {
        let page = try JSONDecoding.agent.decode([WireSample].self, from: Data(FixtureJSON.history.utf8))
        #expect(page.count == 2)
        let sample = try #require(page.map(\.sample).max(by: { $0.at < $1.at }))
        #expect(sample.hostname == "node-a")
        #expect(sample.online)
        #expect(sample.cores == 32)
        #expect(abs(sample.cpuPercent - 1.5) < 0.001)
        #expect(sample.memTotal == 67_196_661_760)
        #expect(sample.networkInterface == "enp6s0")
    }

    @Test("memory use comes from the total and the available bytes")
    func memory() throws {
        let page = try JSONDecoding.agent.decode([WireSample].self, from: Data(FixtureJSON.history.utf8))
        let sample = page[0].sample
        #expect(sample.memoryUsedBytes == 67_196_661_760 - 62_230_171_648)
        let percent = try #require(sample.memoryPercent)
        #expect(abs(percent - 7.39) < 0.05)
    }

    @Test("memory percent is unknown when the total is zero")
    func unknownMemory() {
        let empty = Sample.empty
        #expect(empty.memoryPercent == nil)
        #expect(empty.primaryDisk == nil)
    }

    @Test("the primary disk is the root mount, not the first mount")
    func primaryDisk() throws {
        let page = try JSONDecoding.agent.decode([WireSample].self, from: Data(FixtureJSON.history.utf8))
        let disk = try #require(page[0].sample.primaryDisk)
        #expect(disk.mount == "/")
        #expect(disk.total == 105_021_104_128)
    }

    @Test("the orchestrator snapshot decodes every field the screen shows")
    func orchestrator() throws {
        let snapshot = try JSONDecoding.orchestrator.decode(
            OrchestratorSnapshot.self, from: Data(FixtureJSON.orchestratorStatus.utf8))
        #expect(snapshot.mode == "fast")
        #expect(snapshot.healthy)
        #expect(snapshot.costIsEstimate)
        #expect(snapshot.auth.mode == "subscription")
        #expect(snapshot.accountLabel == "codex pro")
        #expect(snapshot.costText == "est ~$7.15/$7.50 day")
        #expect(snapshot.totals.blocked == 1)
        #expect(snapshot.agents.count == 1)
        #expect(snapshot.agents[0].issue == 91)
        #expect(snapshot.agents[0].title == "fix the parser")
        #expect(snapshot.agents[0].children == nil)
        #expect(snapshot.recent[0].title == nil)
        #expect(snapshot.limits?.weekly?.usedPercent == 84)
        #expect(snapshot.limits?.fiveHour == nil)
        #expect(snapshot.disk?.freeBytes == 77_544_828_928)
    }

    @Test("a real cost is not marked as an estimate")
    func realCost() throws {
        let json = FixtureJSON.orchestratorStatus
            .replacingOccurrences(of: "\"cost_is_estimate\":true", with: "\"cost_is_estimate\":false")
            .replacingOccurrences(of: "\"mode\":\"subscription\"", with: "\"mode\":\"api_key\"")
        let snapshot = try JSONDecoding.orchestrator.decode(
            OrchestratorSnapshot.self, from: Data(json.utf8))
        #expect(snapshot.costText == "$7.15/$7.50 day")
        #expect(snapshot.accountLabel == "api key")
    }

    @Test("a snapshot with missing keys keeps safe defaults instead of failing")
    func partialSnapshot() throws {
        let snapshot = try JSONDecoding.orchestrator.decode(
            OrchestratorSnapshot.self, from: Data("{\"mode\":\"paused\"}".utf8))
        #expect(snapshot.mode == "paused")
        #expect(snapshot.agents.isEmpty)
        #expect(snapshot.recent.isEmpty)
        #expect(snapshot.limits == nil)
        #expect(snapshot.disk == nil)
        #expect(snapshot.healthy == false)
        #expect(snapshot.auth.mode == "unknown")
    }
}
