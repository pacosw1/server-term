import Foundation
import Testing
@testable import ServtermKit

/// The real payload of the new agent. A host that reports none of a kind
/// sends null, not an empty list.
private let sensorBody = """
[{"version":1,"node_id":"node-a","sample":{
  "At":"2026-08-21T05:20:53Z","Online":true,"Hostname":"node-a",
  "NetRx":460368015617,"NetTx":644505220527,
  "Disks":[{"Mount":"/","Device":"/dev/nvme0n1","FSType":"ext4","Total":100,"Used":50},
           {"Mount":"/data","Device":"/dev/nvme1n1p2","FSType":"ext4","Total":100,"Used":10}],
  "Temperatures":[{"Label":"acpitz temp1","Celsius":27.8},
                  {"Label":"coretemp Package id 0","Celsius":61.0},
                  {"Label":"nvme Composite","Celsius":40.85}],
  "DiskIO":[{"Device":"nvme1n1","ReadBytes":2403134430720,"WriteBytes":6230865521664,
             "ReadRate":0,"WriteRate":4552.95},
            {"Device":"nvme0n1","ReadBytes":100,"WriteBytes":200,
             "ReadRate":9000,"WriteRate":10}],
  "Interfaces":[{"Name":"veth1","Rx":10,"Tx":10,"RxRate":0,"TxRate":0,
                 "RxErrors":0,"TxErrors":0,"RxDrops":0,"TxDrops":0},
                {"Name":"enp6s0","Rx":436857873998,"Tx":17909895891,
                 "RxRate":1284.47,"TxRate":1170.84,
                 "RxErrors":0,"TxErrors":0,"RxDrops":0,"TxDrops":2},
                {"Name":"docker0","Rx":500,"Tx":500,"RxRate":10,"TxRate":10,
                 "RxErrors":0,"TxErrors":0,"RxDrops":0,"TxDrops":0}]}}]
"""

/// A macOS host reports null for the two kinds it cannot read.
private let nullBody = """
[{"version":1,"node_id":"mac","sample":{
  "At":"2026-08-21T05:20:53Z","Online":true,"Hostname":"mac",
  "Temperatures":null,"DiskIO":null,
  "Interfaces":[{"Name":"utun0","Rx":0,"Tx":100,"RxRate":0,"TxRate":0,
                 "RxErrors":0,"TxErrors":0,"RxDrops":0,"TxDrops":0}]}}]
"""

@Suite("Sensors, disk IO and interfaces")
struct SensorTests {
    private func sample(_ body: String) throws -> Sample {
        try JSONDecoding.agent.decode([WireSample].self, from: Data(body.utf8))[0].sample
    }

    @Test("the three new lists decode")
    func decode() throws {
        let sample = try sample(sensorBody)
        #expect(sample.temperatures.count == 3)
        #expect(sample.temperatures[0].label == "acpitz temp1")
        #expect(abs(sample.temperatures[0].celsius - 27.8) < 0.001)
        #expect(sample.diskIO.count == 2)
        #expect(sample.diskIO[0].device == "nvme1n1")
        #expect(sample.diskIO[0].writeBytes == 6_230_865_521_664)
        #expect(sample.interfaces.count == 3)
        #expect(sample.interfaces[1].name == "enp6s0")
        #expect(sample.interfaces[1].rx == 436_857_873_998)
        #expect(sample.interfaces[1].txDrops == 2)
    }

    @Test("a null list means the host reports none, and is not a failure")
    func nullMeansNone() throws {
        let sample = try sample(nullBody)
        #expect(sample.temperatures.isEmpty)
        #expect(sample.diskIO.isEmpty)
        #expect(sample.interfaces.count == 1)
        #expect(sample.hasSensors == false)
        #expect(sample.hasDiskIO == false)
    }

    @Test("the aggregate counters hold the corrected byte totals")
    func correctedTotals() throws {
        let sample = try sample(sensorBody)
        #expect(sample.netRx == 460_368_015_617)
        #expect(sample.netTx == 644_505_220_527)
    }

    @Test("the interfaces sort by traffic, the busiest first")
    func interfaceOrder() throws {
        let sorted = InterfaceEntry.sortedByTraffic(try sample(sensorBody).interfaces)
        #expect(sorted.map(\.name) == ["enp6s0", "docker0", "veth1"])
    }

    @Test("a quiet interface goes behind the disclosure, a busy one stays")
    func busySplit() throws {
        let split = InterfaceEntry.split(try sample(sensorBody).interfaces, limit: 5)
        #expect(split.busy.map(\.name) == ["enp6s0", "docker0"])
        #expect(split.rest.map(\.name) == ["veth1"])
    }

    @Test("the busy list keeps the limit, and the rest holds everything else")
    func busyLimit() throws {
        let split = InterfaceEntry.split(try sample(sensorBody).interfaces, limit: 1)
        #expect(split.busy.map(\.name) == ["enp6s0"])
        #expect(split.rest.map(\.name) == ["docker0", "veth1"])
    }

    @Test("an interface with an error or a drop is marked, a clean one is not")
    func faults() throws {
        let interfaces = try sample(sensorBody).interfaces
        #expect(interfaces.first { $0.name == "enp6s0" }?.hasFaults == true)
        #expect(interfaces.first { $0.name == "docker0" }?.hasFaults == false)
    }

    @Test("the disk IO sorts by the busiest device")
    func diskOrder() throws {
        let sorted = DiskIOEntry.sortedByRate(try sample(sensorBody).diskIO)
        #expect(sorted.map(\.device) == ["nvme0n1", "nvme1n1"])
    }

    @Test("a device matches a mount only on the exact device name")
    func mountMatch() throws {
        let sample = try sample(sensorBody)
        #expect(DiskIOEntry.mount(forDevice: "nvme0n1", in: sample.disks) == "/")
        // /dev/nvme1n1p2 is a partition, not the device, so there is no
        // mapping to invent here.
        #expect(DiskIOEntry.mount(forDevice: "nvme1n1", in: sample.disks) == nil)
        #expect(DiskIOEntry.mount(forDevice: "sda", in: sample.disks) == nil)
    }

    @Test("the temperatures sort with the hottest first")
    func temperatureOrder() throws {
        let sorted = Temperature.sortedByHeat(try sample(sensorBody).temperatures)
        #expect(sorted.map(\.label) == ["coretemp Package id 0", "nvme Composite", "acpitz temp1"])
        #expect(Format.celsius(sorted[0].celsius) == "61.0 °C")
    }

    @Test("a rate that the app cannot know yet shows a dash, never a zero")
    func unknownRate() {
        #expect(Format.rate(bytesPerSecond: 0, known: true) == "0 B/s")
        #expect(Format.rate(bytesPerSecond: 1024, known: true) == "1.0 KB/s")
        #expect(Format.rate(bytesPerSecond: 0, known: false) == Format.unknown)
        #expect(Format.rate(bytesPerSecond: 1024, known: false) == Format.unknown)
    }
}
