import Foundation

/// The orchestrator daemon writes snake case keys. JSONDecoding.orchestrator
/// converts them, so every property here uses the matching camel case name.
/// A nil value always means "the daemon reports no reading". A screen must
/// show the unknown mark for it, never a zero.

public struct OrchestratorDaemon: Decodable, Sendable, Equatable {
    public var pid: Int = 0
    public var cpuPercent: Double = 0
    public var rssBytes: Int64 = 0
    public var uptimeSeconds: Int64 = 0

    enum CodingKeys: String, CodingKey {
        case pid, cpuPercent, rssBytes, uptimeSeconds
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        pid = c.or(.pid, 0)
        cpuPercent = c.or(.cpuPercent, 0)
        rssBytes = c.or(.rssBytes, 0)
        uptimeSeconds = c.or(.uptimeSeconds, 0)
    }
}

public struct OrchestratorBudget: Decodable, Sendable, Equatable {
    public var hourUsd: Double = 0
    public var dayUsd: Double = 0
    public var weekUsd: Double = 0
    public var hourLimitUsd: Double = 0
    public var dayLimitUsd: Double = 0
    public var weekLimitUsd: Double = 0
    public var dayRemainingUsd: Double = 0
    public var paceNote: String = ""

    enum CodingKeys: String, CodingKey {
        case hourUsd, dayUsd, weekUsd, hourLimitUsd, dayLimitUsd, weekLimitUsd
        case dayRemainingUsd, paceNote
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        hourUsd = c.or(.hourUsd, 0)
        dayUsd = c.or(.dayUsd, 0)
        weekUsd = c.or(.weekUsd, 0)
        hourLimitUsd = c.or(.hourLimitUsd, 0)
        dayLimitUsd = c.or(.dayLimitUsd, 0)
        weekLimitUsd = c.or(.weekLimitUsd, 0)
        dayRemainingUsd = c.or(.dayRemainingUsd, 0)
        paceNote = c.or(.paceNote, "")
    }
}

public struct OrchestratorTotals: Decodable, Sendable, Equatable {
    public var inputTokens: Int64 = 0
    public var outputTokens: Int64 = 0
    public var costUsd: Double = 0
    public var live: Int = 0
    public var done: Int = 0
    public var blocked: Int = 0

    enum CodingKeys: String, CodingKey {
        case inputTokens, outputTokens, costUsd, live, done, blocked
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        inputTokens = c.or(.inputTokens, 0)
        outputTokens = c.or(.outputTokens, 0)
        costUsd = c.or(.costUsd, 0)
        live = c.or(.live, 0)
        done = c.or(.done, 0)
        blocked = c.or(.blocked, 0)
    }
}

public struct OrchestratorTask: Decodable, Sendable, Hashable, Identifiable {
    public let text: String
    public let done: Bool
    public var id: String { text }
}

public struct OrchestratorChild: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let model: String
    public let state: String
    public let task: String
    public let elapsedSeconds: Int64
    public let inputTokens: Int64
    public let outputTokens: Int64
    public let pid: Int?
    public let exitCode: Int?
}

public struct OrchestratorAgent: Decodable, Sendable, Hashable, Identifiable {
    public var issue: Int = 0
    public var title: String?
    public var state: String = ""
    public var cycle: Int = 0
    public var prNumber: Int?
    public var branch: String = ""
    public var elapsedSeconds: Int64 = 0
    public var inputTokens: Int64 = 0
    public var outputTokens: Int64 = 0
    public var costUsd: Double = 0
    public var pid: Int = 0
    public var cpuPercent: Double = 0
    public var rssBytes: Int64 = 0
    public var lastError: String = ""
    public var weeklyPercentUsed: Double?
    public var lastActivity: String?
    public var activityAgeSeconds: Int64?
    public var turns: Int = 0
    /// children is nil when the daemon does not report subagents yet. That
    /// is not the same as an empty list.
    public var children: [OrchestratorChild]?
    public var childrenRunning: Int = 0
    public var childrenDone: Int = 0
    public var childrenFailed: Int = 0
    public var worktree: String = ""
    public var worktreeDiskBytes: Int64?
    public var tasks: [OrchestratorTask]?

    public var id: Int { issue }
    public var displayTitle: String { title ?? "issue \(issue)" }

    /// issueURL points at the issue page. It is nil when the daemon
    /// reports no repository, so no screen shows a link that fails.
    public func issueURL(repo: String) -> URL? {
        guard !repo.isEmpty, issue > 0 else { return nil }
        return URL(string: "https://github.com/\(repo)/issues/\(issue)")
    }

    /// pullRequestURL points at the pull request. It is nil until the
    /// agent opens one.
    public func pullRequestURL(repo: String) -> URL? {
        guard !repo.isEmpty, let number = prNumber, number > 0 else { return nil }
        return URL(string: "https://github.com/\(repo)/pull/\(number)")
    }

    enum CodingKeys: String, CodingKey {
        case issue, title, state, cycle, prNumber, branch, elapsedSeconds, inputTokens
        case outputTokens, costUsd, pid, cpuPercent, rssBytes, lastError
        case weeklyPercentUsed, lastActivity, activityAgeSeconds, turns, children
        case childrenRunning, childrenDone, childrenFailed, worktree, worktreeDiskBytes
        case tasks
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        issue = c.or(.issue, 0)
        title = c.maybe(.title)
        state = c.or(.state, "")
        cycle = c.or(.cycle, 0)
        prNumber = c.maybe(.prNumber)
        branch = c.or(.branch, "")
        elapsedSeconds = c.or(.elapsedSeconds, 0)
        inputTokens = c.or(.inputTokens, 0)
        outputTokens = c.or(.outputTokens, 0)
        costUsd = c.or(.costUsd, 0)
        pid = c.or(.pid, 0)
        cpuPercent = c.or(.cpuPercent, 0)
        rssBytes = c.or(.rssBytes, 0)
        lastError = c.or(.lastError, "")
        weeklyPercentUsed = c.maybe(.weeklyPercentUsed)
        lastActivity = c.maybe(.lastActivity)
        activityAgeSeconds = c.maybe(.activityAgeSeconds)
        turns = c.or(.turns, 0)
        children = c.maybe(.children)
        childrenRunning = c.or(.childrenRunning, 0)
        childrenDone = c.or(.childrenDone, 0)
        childrenFailed = c.or(.childrenFailed, 0)
        worktree = c.or(.worktree, "")
        worktreeDiskBytes = c.maybe(.worktreeDiskBytes)
        tasks = c.maybe(.tasks)
    }
}

public struct OrchestratorUsageWindow: Decodable, Sendable, Equatable {
    public let usedPercent: Double
    public let resetsAt: Int64
}

public struct OrchestratorLimits: Decodable, Sendable, Equatable {
    public var weekly: OrchestratorUsageWindow?
    public var fiveHour: OrchestratorUsageWindow?
    public var planType: String = ""

    enum CodingKeys: String, CodingKey {
        case weekly, fiveHour, planType
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        weekly = c.maybe(.weekly)
        fiveHour = c.maybe(.fiveHour)
        planType = c.or(.planType, "")
    }
}

public struct OrchestratorRecent: Decodable, Sendable, Equatable, Identifiable {
    public var issue: Int = 0
    public var state: String = ""
    public var prNumber: Int?
    public var costUsd: Double = 0
    public var title: String?
    public var lastError: String = ""

    public var id: Int { issue }
    public var displayTitle: String { title ?? "issue \(issue)" }

    /// issueURL points at the issue page. It is nil when the daemon
    /// reports no repository, so no screen shows a link that fails.
    public func issueURL(repo: String) -> URL? {
        guard !repo.isEmpty, issue > 0 else { return nil }
        return URL(string: "https://github.com/\(repo)/issues/\(issue)")
    }

    /// pullRequestURL points at the pull request. It is nil until the
    /// agent opens one.
    public func pullRequestURL(repo: String) -> URL? {
        guard !repo.isEmpty, let number = prNumber, number > 0 else { return nil }
        return URL(string: "https://github.com/\(repo)/pull/\(number)")
    }

    enum CodingKeys: String, CodingKey {
        case issue, state, prNumber, costUsd, title, lastError
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        issue = c.or(.issue, 0)
        state = c.or(.state, "")
        prNumber = c.maybe(.prNumber)
        costUsd = c.or(.costUsd, 0)
        title = c.maybe(.title)
        lastError = c.or(.lastError, "")
    }
}

public struct OrchestratorDisk: Decodable, Sendable, Equatable {
    public let totalBytes: Int64
    public let freeBytes: Int64
    public let usedBytes: Int64

    /// usedPercent is nil when the daemon reports no size.
    public var usedPercent: Double? {
        totalBytes == 0 ? nil : Double(usedBytes) / Double(totalBytes) * 100
    }
}

public struct OrchestratorAuth: Decodable, Sendable, Equatable {
    public var mode: String = "unknown"
    public var planType: String = ""
    public var billed: Bool = false

    enum CodingKeys: String, CodingKey {
        case mode, planType, billed
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        mode = c.or(.mode, "unknown")
        planType = c.or(.planType, "")
        billed = c.or(.billed, false)
    }
}

public struct OrchestratorSnapshot: Decodable, Sendable, Equatable {
    public var schemaVersion: Int = 0
    public var name: String = ""
    public var at: Date = .distantPast
    public var healthy: Bool = false
    public var mode: String = ""
    public var repo: String = ""
    public var daemon = OrchestratorDaemon()
    public var budget = OrchestratorBudget()
    public var totals = OrchestratorTotals()
    public var agents: [OrchestratorAgent] = []
    public var recent: [OrchestratorRecent] = []
    public var limits: OrchestratorLimits?
    public var disk: OrchestratorDisk?
    public var auth = OrchestratorAuth()
    /// costIsEstimate is true when the daemon computes the dollar figures
    /// from token use. The app must then mark them as an estimate.
    public var costIsEstimate: Bool = false
    public var error: String = ""

    /// accountLabel names the account that pays for the tokens.
    public var accountLabel: String {
        switch auth.mode {
        case "subscription":
            return auth.planType.isEmpty ? "codex" : "codex " + auth.planType
        case "api_key":
            return "api key"
        default:
            return "unknown account"
        }
    }

    /// costText shows the day spend against the day limit. It marks a
    /// computed figure as an estimate, because it is not a real charge.
    public var costText: String {
        let amounts = String(format: "$%.2f/$%.2f day", budget.dayUsd, budget.dayLimitUsd)
        if costIsEstimate { return "est ~" + amounts }
        if auth.mode == "unknown" { return amounts + " billed" }
        return amounts
    }

    enum CodingKeys: String, CodingKey {
        case schemaVersion, name, at, healthy, mode, repo, daemon, budget, totals, agents
        case recent, limits, disk, auth, costIsEstimate, error
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = c.or(.schemaVersion, 0)
        name = c.or(.name, "")
        at = c.or(.at, Date.distantPast)
        healthy = c.or(.healthy, false)
        mode = c.or(.mode, "")
        repo = c.or(.repo, "")
        daemon = c.or(.daemon, OrchestratorDaemon())
        budget = c.or(.budget, OrchestratorBudget())
        totals = c.or(.totals, OrchestratorTotals())
        agents = c.or(.agents, [])
        recent = c.or(.recent, [])
        limits = c.maybe(.limits)
        disk = c.maybe(.disk)
        auth = c.or(.auth, OrchestratorAuth())
        costIsEstimate = c.or(.costIsEstimate, false)
        error = c.or(.error, "")
    }
}
