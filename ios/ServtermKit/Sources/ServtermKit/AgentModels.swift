import Foundation

/// ServtermDate reads the timestamps that the Go agent writes. Go writes
/// RFC 3339 with up to nine fraction digits and with a numeric zone offset.
/// Foundation reads at most three fraction digits, so the parser cuts the
/// fraction first.
public enum ServtermDate {
    // A date formatter is safe to share between threads after the setup,
    // so one shared instance avoids a rebuild on every decoded date.
    nonisolated(unsafe) private static let withFraction: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()

    nonisolated(unsafe) private static let plain: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter
    }()

    public static func parse(_ text: String) -> Date? {
        let short = shortenFraction(text)
        if let date = withFraction.date(from: short) { return date }
        return plain.date(from: short)
    }

    /// shortenFraction keeps at most three digits after the decimal point.
    private static func shortenFraction(_ text: String) -> String {
        guard let dot = text.firstIndex(of: ".") else { return text }
        var digits = text.index(after: dot)
        var count = 0
        while digits < text.endIndex, text[digits].isNumber {
            digits = text.index(after: digits)
            count += 1
        }
        if count <= 3 { return text }
        let keep = text.index(dot, offsetBy: 4)
        return String(text[text.startIndex..<keep]) + String(text[digits...])
    }
}

/// JSONDecoding holds the two decoders that the app needs. The agent writes
/// Go field names. The orchestrator daemon writes snake case names.
public enum JSONDecoding {
    private static let dateStrategy = JSONDecoder.DateDecodingStrategy.custom { decoder in
        let text = try decoder.singleValueContainer().decode(String.self)
        guard let date = ServtermDate.parse(text) else {
            throw DecodingError.dataCorrupted(
                .init(codingPath: decoder.codingPath, debugDescription: "bad date: \(text)"))
        }
        return date
    }

    public static var agent: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = dateStrategy
        return decoder
    }

    public static var orchestrator: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = dateStrategy
        return decoder
    }
}

/// AgentStatus is the answer of the unauthenticated GET /v1/status endpoint.
public struct AgentStatus: Decodable, Sendable, Equatable {
    public let service: String
    public let version: Int
    public let nodeID: String
    public let latestAt: Date?

    enum CodingKeys: String, CodingKey {
        case service, version
        case nodeID = "node_id"
        case latestAt = "latest_at"
    }
}

/// DiskEntry is one mounted file system.
public struct DiskEntry: Decodable, Sendable, Equatable, Identifiable {
    public let mount: String
    public let device: String
    public let fsType: String
    public let total: UInt64
    public let used: UInt64

    public var id: String { mount + device }
    public var freeBytes: UInt64 { total > used ? total - used : 0 }
    /// usedPercent is nil when the size is unknown, so no screen draws a
    /// full or an empty bar for a disk that never reported a size.
    public var usedPercent: Double? {
        total == 0 ? nil : Double(used) / Double(total) * 100
    }

    enum CodingKeys: String, CodingKey {
        case mount = "Mount", device = "Device", fsType = "FSType"
        case total = "Total", used = "Used"
    }
}

/// ProcessEntry is one process in the top list.
public struct ProcessEntry: Decodable, Sendable, Equatable, Identifiable {
    public let pid: Int
    public let user: String
    public let command: String
    public let cpu: Double
    public let memory: Double
    public let rss: UInt64

    public var id: Int { pid }

    enum CodingKeys: String, CodingKey {
        case pid = "PID", user = "User", command = "Command"
        case cpu = "CPU", memory = "Memory", rss = "RSS"
    }
}

/// AcceleratorEntry is one GPU or NPU.
public struct AcceleratorEntry: Decodable, Sendable, Equatable, Identifiable {
    public let kind: String
    public let name: String
    public let utilization: Double
    public let utilizationKnown: Bool

    public var id: String { kind + name }
    /// utilizationPercent is nil when the driver reports no reading.
    public var utilizationPercent: Double? { utilizationKnown ? utilization : nil }

    enum CodingKeys: String, CodingKey {
        case kind = "Kind", name = "Name"
        case utilization = "Utilization", utilizationKnown = "UtilizationKnown"
    }
}

/// Sample is the subset of the agent reading that the app shows. The agent
/// marshals its Go struct without JSON tags, so the keys are the Go field
/// names.
public struct Sample: Decodable, Sendable, Equatable {
    public var at: Date = .distantPast
    public var online: Bool = false
    public var error: String = ""
    public var hostname: String = ""
    public var os: String = ""
    public var kernel: String = ""
    public var uptimeSeconds: Double = 0
    public var cpuPercent: Double = 0
    public var cores: Int = 0
    public var load1: Double = 0
    public var load5: Double = 0
    public var load15: Double = 0
    public var memTotal: UInt64 = 0
    public var memAvailable: UInt64 = 0
    public var swapTotal: UInt64 = 0
    public var swapFree: UInt64 = 0
    public var netRxRate: Double = 0
    public var netTxRate: Double = 0
    public var networkInterface: String = ""
    public var networkType: String = ""
    public var networkLinkMbps: Int = 0
    public var powerWatts: Double = 0
    public var powerKnown: Bool = false
    public var batteryPercent: Double = 0
    public var batteryKnown: Bool = false
    public var batteryCharging: Bool = false
    public var disks: [DiskEntry] = []
    public var accelerators: [AcceleratorEntry] = []
    public var processes: [ProcessEntry] = []

    public static let empty = Sample()

    enum CodingKeys: String, CodingKey {
        case at = "At", online = "Online", error = "Error"
        case hostname = "Hostname", os = "OS", kernel = "Kernel"
        case uptimeSeconds = "UptimeSeconds", cpuPercent = "CPUPercent", cores = "Cores"
        case load1 = "Load1", load5 = "Load5", load15 = "Load15"
        case memTotal = "MemTotal", memAvailable = "MemAvailable"
        case swapTotal = "SwapTotal", swapFree = "SwapFree"
        case netRxRate = "NetRxRate", netTxRate = "NetTxRate"
        case networkInterface = "NetworkInterface", networkType = "NetworkType"
        case networkLinkMbps = "NetworkLinkMbps"
        case powerWatts = "PowerWatts", powerKnown = "PowerKnown"
        case batteryPercent = "BatteryPercent", batteryKnown = "BatteryKnown"
        case batteryCharging = "BatteryCharging"
        case disks = "Disks", accelerators = "Accelerators", processes = "Processes"
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        at = try container.decodeIfPresent(Date.self, forKey: .at) ?? .distantPast
        online = try container.decodeIfPresent(Bool.self, forKey: .online) ?? false
        error = try container.decodeIfPresent(String.self, forKey: .error) ?? ""
        hostname = try container.decodeIfPresent(String.self, forKey: .hostname) ?? ""
        os = try container.decodeIfPresent(String.self, forKey: .os) ?? ""
        kernel = try container.decodeIfPresent(String.self, forKey: .kernel) ?? ""
        uptimeSeconds = try container.decodeIfPresent(Double.self, forKey: .uptimeSeconds) ?? 0
        cpuPercent = try container.decodeIfPresent(Double.self, forKey: .cpuPercent) ?? 0
        cores = try container.decodeIfPresent(Int.self, forKey: .cores) ?? 0
        load1 = try container.decodeIfPresent(Double.self, forKey: .load1) ?? 0
        load5 = try container.decodeIfPresent(Double.self, forKey: .load5) ?? 0
        load15 = try container.decodeIfPresent(Double.self, forKey: .load15) ?? 0
        memTotal = try container.decodeIfPresent(UInt64.self, forKey: .memTotal) ?? 0
        memAvailable = try container.decodeIfPresent(UInt64.self, forKey: .memAvailable) ?? 0
        swapTotal = try container.decodeIfPresent(UInt64.self, forKey: .swapTotal) ?? 0
        swapFree = try container.decodeIfPresent(UInt64.self, forKey: .swapFree) ?? 0
        netRxRate = try container.decodeIfPresent(Double.self, forKey: .netRxRate) ?? 0
        netTxRate = try container.decodeIfPresent(Double.self, forKey: .netTxRate) ?? 0
        networkInterface = try container.decodeIfPresent(String.self, forKey: .networkInterface) ?? ""
        networkType = try container.decodeIfPresent(String.self, forKey: .networkType) ?? ""
        networkLinkMbps = try container.decodeIfPresent(Int.self, forKey: .networkLinkMbps) ?? 0
        powerWatts = try container.decodeIfPresent(Double.self, forKey: .powerWatts) ?? 0
        powerKnown = try container.decodeIfPresent(Bool.self, forKey: .powerKnown) ?? false
        batteryPercent = try container.decodeIfPresent(Double.self, forKey: .batteryPercent) ?? 0
        batteryKnown = try container.decodeIfPresent(Bool.self, forKey: .batteryKnown) ?? false
        batteryCharging = try container.decodeIfPresent(Bool.self, forKey: .batteryCharging) ?? false
        disks = try container.decodeIfPresent([DiskEntry].self, forKey: .disks) ?? []
        accelerators = try container.decodeIfPresent([AcceleratorEntry].self, forKey: .accelerators) ?? []
        processes = try container.decodeIfPresent([ProcessEntry].self, forKey: .processes) ?? []
    }

    public var memoryUsedBytes: UInt64 {
        memTotal > memAvailable ? memTotal - memAvailable : 0
    }

    /// memoryPercent is nil when the agent reports no memory size. A screen
    /// must show the unknown mark then, never 0%.
    public var memoryPercent: Double? {
        memTotal == 0 ? nil : Double(memoryUsedBytes) / Double(memTotal) * 100
    }

    public var swapUsedBytes: UInt64 {
        swapTotal > swapFree ? swapTotal - swapFree : 0
    }

    /// primaryDisk is the root mount. It falls back to the largest real
    /// file system, and it is nil when the agent reports no disk.
    public var primaryDisk: DiskEntry? {
        if let root = disks.first(where: { $0.mount == "/" || $0.mount == "C:\\" }) { return root }
        return disks.filter { $0.total > 0 }.max(by: { $0.total < $1.total })
    }

    /// batteryLevel is nil when the machine reports no battery.
    public var batteryLevel: Double? { batteryKnown ? batteryPercent : nil }

    /// power is nil when the machine reports no power meter.
    public var power: Double? { powerKnown ? powerWatts : nil }

    public var topProcesses: [ProcessEntry] {
        processes.sorted { $0.cpu > $1.cpu }
    }
}

/// WireSample is the versioned envelope that the agent sends.
public struct WireSample: Decodable, Sendable, Equatable {
    public let version: Int
    public let nodeID: String
    public let sample: Sample

    enum CodingKeys: String, CodingKey {
        case version
        case nodeID = "node_id"
        case sample
    }
}
