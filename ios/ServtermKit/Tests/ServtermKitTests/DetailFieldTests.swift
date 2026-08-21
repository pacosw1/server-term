import Foundation
import Testing
@testable import ServtermKit

/// These keys all come from metrics.Sample in the Go agent. The app must
/// not show a field that the Go struct does not carry.
private let detailBody = """
[{"version":1,"node_id":"node-a","sample":{
  "At":"2026-08-21T05:20:53Z","Online":true,"Latency":153404704,
  "Hostname":"node-a","Cores":4,
  "CorePercent":[1.5,0,50,99.5],
  "PressureCPU":0.42,"PressureMemory":0,"PressureIO":1.75,
  "NetRx":460368015617,"NetTx":644505220527,
  "NetRxErrors":0,"NetTxErrors":45,"NetRxDrops":3,"NetTxDrops":0,
  "Devices":[{"Name":"nvme0n1","Kind":"ssd","Size":1920383410176},
             {"Name":"md0","Kind":"ssd","Size":268369920}]}}]
"""

@Suite("Detail fields")
struct DetailFieldTests {
    private func sample() throws -> Sample {
        try JSONDecoding.agent.decode([WireSample].self, from: Data(detailBody.utf8))[0].sample
    }

    @Test("the per core readings decode in order")
    func cores() throws {
        let sample = try sample()
        #expect(sample.corePercent.count == 4)
        #expect(sample.corePercent[2] == 50)
        #expect(sample.corePercent[3] == 99.5)
    }

    @Test("a sample without per core readings shows none")
    func noCores() {
        #expect(Sample.empty.corePercent.isEmpty)
    }

    @Test("the pressure readings decode, and a host without them shows none")
    func pressure() throws {
        let sample = try sample()
        #expect(sample.hasPressure)
        #expect(abs(sample.pressureCPU - 0.42) < 0.001)
        #expect(abs(sample.pressureIO - 1.75) < 0.001)
        #expect(Sample.empty.hasPressure == false)
    }

    @Test("the block devices decode with their kind and size")
    func devices() throws {
        let devices = try sample().devices
        #expect(devices.count == 2)
        #expect(devices[0].name == "nvme0n1")
        #expect(devices[0].kind == "ssd")
        #expect(devices[0].size == 1_920_383_410_176)
    }

    @Test("the network counters decode, and the totals stay whole")
    func networkCounters() throws {
        let sample = try sample()
        #expect(sample.netRx == 460_368_015_617)
        #expect(sample.netTx == 644_505_220_527)
        #expect(sample.netTxErrors == 45)
        #expect(sample.netRxDrops == 3)
        #expect(sample.hasNetworkFaults)
    }

    @Test("a clean interface reports no fault")
    func noFaults() {
        #expect(Sample.empty.hasNetworkFaults == false)
    }

    @Test("the agent latency reads as seconds, from the Go nanoseconds")
    func latency() throws {
        let sample = try sample()
        let seconds = try #require(sample.latencySeconds)
        #expect(abs(seconds - 0.1534) < 0.0001)
        // A sample with no latency reading shows a dash, not a zero.
        #expect(Sample.empty.latencySeconds == nil)
    }

    @Test("the process list sorts by CPU or by memory")
    func processSorting() {
        var sample = Sample.empty
        sample.processes = [
            ProcessEntry(pid: 1, user: "a", command: "low-cpu", cpu: 1, memory: 9, rss: 900),
            ProcessEntry(pid: 2, user: "b", command: "high-cpu", cpu: 80, memory: 1, rss: 100),
        ]
        #expect(sample.processes(sortedBy: .cpu).first?.command == "high-cpu")
        #expect(sample.processes(sortedBy: .memory).first?.command == "low-cpu")
    }

    @Test("the job list sorts by elapsed, longest first")
    func jobSorting() {
        let now = Date(timeIntervalSince1970: 1000)
        var young = RunnerJob()
        young.workerPID = 1
        young.startedAt = Date(timeIntervalSince1970: 990)
        var old = RunnerJob()
        old.workerPID = 2
        old.startedAt = Date(timeIntervalSince1970: 100)
        #expect(RunnerJob.sortedByElapsed([young, old], now: now).map(\.workerPID) == [2, 1])
    }

}
