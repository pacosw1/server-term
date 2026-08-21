import Foundation

/// Temperature is one sensor reading. The agent sends no limit with it, so
/// the app shows the number and never invents a hot threshold.
public struct Temperature: Decodable, Sendable, Equatable, Identifiable {
    public let label: String
    public let celsius: Double

    public var id: String { label }

    enum CodingKeys: String, CodingKey {
        case label = "Label", celsius = "Celsius"
    }

    /// sortedByHeat puts the hottest sensor first.
    public static func sortedByHeat(_ sensors: [Temperature]) -> [Temperature] {
        sensors.sorted { first, second in
            first.celsius == second.celsius ? first.label < second.label : first.celsius > second.celsius
        }
    }
}

/// DiskIOEntry is the traffic of one block device. The rates come from two
/// samples, so they are 0 on the first reading after a device appears.
public struct DiskIOEntry: Decodable, Sendable, Equatable, Identifiable {
    public let device: String
    public let readBytes: UInt64
    public let writeBytes: UInt64
    public let readRate: Double
    public let writeRate: Double

    public var id: String { device }
    public var totalRate: Double { readRate + writeRate }

    enum CodingKeys: String, CodingKey {
        case device = "Device", readBytes = "ReadBytes", writeBytes = "WriteBytes"
        case readRate = "ReadRate", writeRate = "WriteRate"
    }

    /// sortedByRate puts the busiest device first, and falls back to the
    /// total bytes when two devices are equally quiet.
    public static func sortedByRate(_ entries: [DiskIOEntry]) -> [DiskIOEntry] {
        entries.sorted { first, second in
            if first.totalRate != second.totalRate { return first.totalRate > second.totalRate }
            let firstTotal = first.readBytes + first.writeBytes
            let secondTotal = second.readBytes + second.writeBytes
            if firstTotal != secondTotal { return firstTotal > secondTotal }
            return first.device < second.device
        }
    }

    /// mount names the file system of one device. It matches only when the
    /// device name is exactly the same, so a partition of a device never
    /// borrows the mount of the whole disk.
    public static func mount(forDevice device: String, in disks: [DiskEntry]) -> String? {
        disks.first { disk in
            let name = disk.device.split(separator: "/").last.map(String.init) ?? disk.device
            return name == device
        }?.mount
    }
}

/// InterfaceEntry is one network interface. A machine that runs containers
/// carries many quiet ones, so the screen shows the busy ones first.
public struct InterfaceEntry: Decodable, Sendable, Equatable, Identifiable {
    public let name: String
    public let rx: UInt64
    public let tx: UInt64
    public let rxRate: Double
    public let txRate: Double
    public let rxErrors: UInt64
    public let txErrors: UInt64
    public let rxDrops: UInt64
    public let txDrops: UInt64

    public var id: String { name }
    public var totalRate: Double { rxRate + txRate }
    public var totalBytes: UInt64 { rx + tx }
    /// hasFaults is true when the interface counted any error or any
    /// dropped packet. A clean interface stays quiet on the screen.
    public var hasFaults: Bool {
        rxErrors > 0 || txErrors > 0 || rxDrops > 0 || txDrops > 0
    }

    enum CodingKeys: String, CodingKey {
        case name = "Name", rx = "Rx", tx = "Tx", rxRate = "RxRate", txRate = "TxRate"
        case rxErrors = "RxErrors", txErrors = "TxErrors"
        case rxDrops = "RxDrops", txDrops = "TxDrops"
    }

    public static func sortedByTraffic(_ entries: [InterfaceEntry]) -> [InterfaceEntry] {
        entries.sorted { first, second in
            if first.totalRate != second.totalRate { return first.totalRate > second.totalRate }
            if first.totalBytes != second.totalBytes { return first.totalBytes > second.totalBytes }
            return first.name < second.name
        }
    }

    /// split puts the interfaces that carry traffic in front, up to the
    /// limit, and leaves the rest for a disclosure. A host with 27 docker
    /// pairs then shows the one that matters.
    public static func split(
        _ entries: [InterfaceEntry], limit: Int
    ) -> (busy: [InterfaceEntry], rest: [InterfaceEntry]) {
        let sorted = sortedByTraffic(entries)
        let busy = sorted.prefix { $0.totalRate > 0 }.prefix(max(0, limit))
        return (Array(busy), Array(sorted.dropFirst(busy.count)))
    }
}
