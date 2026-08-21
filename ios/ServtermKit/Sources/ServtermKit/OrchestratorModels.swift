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
}

public struct OrchestratorTotals: Decodable, Sendable, Equatable {
    public var inputTokens: Int64 = 0
    public var outputTokens: Int64 = 0
    public var costUsd: Double = 0
    public var live: Int = 0
    public var done: Int = 0
    public var blocked: Int = 0
}

public struct OrchestratorTask: Decodable, Sendable, Equatable, Identifiable {
    public let text: String
    public let done: Bool
    public var id: String { text }
}

public struct OrchestratorChild: Decodable, Sendable, Equatable, Identifiable {
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

public struct OrchestratorAgent: Decodable, Sendable, Equatable, Identifiable {
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
}

public struct OrchestratorUsageWindow: Decodable, Sendable, Equatable {
    public let usedPercent: Double
    public let resetsAt: Int64
}

public struct OrchestratorLimits: Decodable, Sendable, Equatable {
    public var weekly: OrchestratorUsageWindow?
    public var fiveHour: OrchestratorUsageWindow?
    public var planType: String = ""
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
}
