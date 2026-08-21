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

/// BlockDevice is one disk that the machine carries. The agent reports the
/// name, the kind and the size only.
public struct BlockDevice: Decodable, Sendable, Equatable, Identifiable {
    public let name: String
    public let kind: String
    public let size: UInt64

    public var id: String { name }

    enum CodingKeys: String, CodingKey {
        case name = "Name", kind = "Kind", size = "Size"
    }
}

/// ProcessSort is the order of the process list on the screen.
public enum ProcessSort: String, Sendable, CaseIterable, Identifiable {
    case cpu
    case memory

    public var id: String { rawValue }

    public var label: String {
        switch self {
        case .cpu: return "CPU"
        case .memory: return "Memory"
        }
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

    public init(pid: Int, user: String, command: String, cpu: Double, memory: Double, rss: UInt64) {
        self.pid = pid
        self.user = user
        self.command = command
        self.cpu = cpu
        self.memory = memory
        self.rss = rss
    }

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
    public var corePercent: [Double] = []
    public var netRx: UInt64 = 0
    public var netTx: UInt64 = 0
    public var netRxErrors: UInt64 = 0
    public var netTxErrors: UInt64 = 0
    public var netRxDrops: UInt64 = 0
    public var netTxDrops: UInt64 = 0
    public var pressureCPU: Double = 0
    public var pressureMemory: Double = 0
    public var pressureIO: Double = 0
    public var devices: [BlockDevice] = []
    /// A host that cannot read a kind sends null for it, which means "none
    /// reported". It is a normal state, never a failure.
    public var temperatures: [Temperature] = []
    public var diskIO: [DiskIOEntry] = []
    public var interfaces: [InterfaceEntry] = []
    /// latency is the age of the reading in nanoseconds, as the Go agent
    /// writes a time.Duration.
    public var latency: Int64 = 0
    public var runners = RunnerStats()
    public var runnerJobs: [RunnerJob] = []

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
        case runners = "Runners", runnerJobs = "RunnerJobs"
        case corePercent = "CorePercent", devices = "Devices", latency = "Latency"
        case temperatures = "Temperatures", diskIO = "DiskIO", interfaces = "Interfaces"
        case netRx = "NetRx", netTx = "NetTx"
        case netRxErrors = "NetRxErrors", netTxErrors = "NetTxErrors"
        case netRxDrops = "NetRxDrops", netTxDrops = "NetTxDrops"
        case pressureCPU = "PressureCPU", pressureMemory = "PressureMemory"
        case pressureIO = "PressureIO"
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
        corePercent = try container.decodeIfPresent([Double].self, forKey: .corePercent) ?? []
        devices = try container.decodeIfPresent([BlockDevice].self, forKey: .devices) ?? []
        temperatures = try container.decodeIfPresent([Temperature].self, forKey: .temperatures) ?? []
        diskIO = try container.decodeIfPresent([DiskIOEntry].self, forKey: .diskIO) ?? []
        interfaces = try container.decodeIfPresent([InterfaceEntry].self, forKey: .interfaces) ?? []
        latency = try container.decodeIfPresent(Int64.self, forKey: .latency) ?? 0
        netRx = try container.decodeIfPresent(UInt64.self, forKey: .netRx) ?? 0
        netTx = try container.decodeIfPresent(UInt64.self, forKey: .netTx) ?? 0
        netRxErrors = try container.decodeIfPresent(UInt64.self, forKey: .netRxErrors) ?? 0
        netTxErrors = try container.decodeIfPresent(UInt64.self, forKey: .netTxErrors) ?? 0
        netRxDrops = try container.decodeIfPresent(UInt64.self, forKey: .netRxDrops) ?? 0
        netTxDrops = try container.decodeIfPresent(UInt64.self, forKey: .netTxDrops) ?? 0
        pressureCPU = try container.decodeIfPresent(Double.self, forKey: .pressureCPU) ?? 0
        pressureMemory = try container.decodeIfPresent(Double.self, forKey: .pressureMemory) ?? 0
        pressureIO = try container.decodeIfPresent(Double.self, forKey: .pressureIO) ?? 0
        runners = try container.decodeIfPresent(RunnerStats.self, forKey: .runners) ?? RunnerStats()
        runnerJobs = try container.decodeIfPresent([RunnerJob].self, forKey: .runnerJobs) ?? []
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

    /// sortedDisks puts the root mount first and then the largest file
    /// systems, so the important readings come first on a small screen.
    public var sortedDisks: [DiskEntry] {
        disks.sorted { first, second in
            if first.mount == "/" { return true }
            if second.mount == "/" { return false }
            return first.total > second.total
        }
    }

    /// batteryLevel is nil when the machine reports no battery.
    public var batteryLevel: Double? { batteryKnown ? batteryPercent : nil }

    /// power is nil when the machine reports no power meter.
    public var power: Double? { powerKnown ? powerWatts : nil }

    /// hasRunners says whether this server runs CI runners at all. A server
    /// with a listener but no job still belongs on the runners screen.
    public var hasRunners: Bool {
        runners.listeners > 0 || runners.activeJobs > 0 || !runnerJobs.isEmpty
    }

    public var topProcesses: [ProcessEntry] {
        processes(sortedBy: .cpu)
    }

    /// processes returns the list in the order that the reader chose.
    public func processes(sortedBy order: ProcessSort) -> [ProcessEntry] {
        switch order {
        case .cpu: return processes.sorted { $0.cpu > $1.cpu }
        case .memory: return processes.sorted { $0.rss > $1.rss }
        }
    }

    /// hasSensors says whether the host reads any temperature at all. A
    /// virtual machine and a Mac report none.
    public var hasSensors: Bool { !temperatures.isEmpty }

    /// hasDiskIO says whether the host counts block device traffic.
    public var hasDiskIO: Bool { !diskIO.isEmpty }

    /// hasPressure says whether the host reports the pressure readings at
    /// all. Only Linux does, so a machine without them shows no card.
    public var hasPressure: Bool {
        pressureCPU > 0 || pressureMemory > 0 || pressureIO > 0
    }

    /// hasNetworkFaults says whether the interface counted any error or
    /// any dropped packet.
    public var hasNetworkFaults: Bool {
        netRxErrors > 0 || netTxErrors > 0 || netRxDrops > 0 || netTxDrops > 0
    }

    /// latencySeconds is the age of the reading. It is nil when the agent
    /// reports none, so the screen shows a dash instead of a zero.
    public var latencySeconds: Double? {
        latency <= 0 ? nil : Double(latency) / 1_000_000_000
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
