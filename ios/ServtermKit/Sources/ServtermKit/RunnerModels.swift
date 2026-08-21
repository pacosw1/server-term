import Foundation

/// RunnerStats is the summary of the CI runners on one server. The agent
/// marshals the Go struct without JSON tags, so the keys are the Go field
/// names.
public struct RunnerStats: Decodable, Sendable, Equatable {
    public var listeners: Int = 0
    public var activeJobs: Int = 0
    public var cpu: Double = 0
    public var memory: Double = 0
    public var rss: UInt64 = 0
    public var processes: Int = 0
    public var cpuTicks: UInt64 = 0

    enum CodingKeys: String, CodingKey {
        case listeners = "Listeners", activeJobs = "ActiveJobs", cpu = "CPU"
        case memory = "Memory", rss = "RSS", processes = "Processes", cpuTicks = "CPUTicks"
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        listeners = c.or(.listeners, 0)
        activeJobs = c.or(.activeJobs, 0)
        cpu = c.or(.cpu, 0)
        memory = c.or(.memory, 0)
        rss = c.or(.rss, 0)
        processes = c.or(.processes, 0)
        cpuTicks = c.or(.cpuTicks, 0)
    }

    /// hostShare is the part of the whole machine that the runners use. It
    /// is nil when the agent reports no core count, so no screen shows a
    /// share that the app cannot compute.
    public func hostShare(cores: Int) -> Double? {
        cores == 0 ? nil : cpu / Double(cores)
    }
}

/// RunnerJob is one CI job that runs now. Only worker_pid carries a JSON
/// tag in the agent; the other keys are the Go field names.
public struct RunnerJob: Decodable, Sendable, Hashable, Identifiable {
    public var workerPID: Int = 0
    public var runner: String = ""
    public var repository: String = ""
    public var workflow: String = ""
    public var job: String = ""
    public var runID: String = ""
    public var runNumber: String = ""
    public var serverURL: String = ""
    public var startedAt: Date?
    public var cpuTicks: UInt64 = 0
    public var rss: UInt64 = 0
    public var processes: Int = 0
    public var cpu: Double = 0

    public var id: Int { workerPID }

    enum CodingKeys: String, CodingKey {
        case workerPID = "worker_pid"
        case runner = "Runner", repository = "Repository", workflow = "Workflow"
        case job = "Job", runID = "RunID", runNumber = "RunNumber"
        case serverURL = "ServerURL", startedAt = "StartedAt"
        case cpuTicks = "CPUTicks", rss = "RSS", processes = "Processes", cpu = "CPU"
    }

    public init() {}

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        workerPID = c.or(.workerPID, 0)
        runner = c.or(.runner, "")
        repository = c.or(.repository, "")
        workflow = c.or(.workflow, "")
        job = c.or(.job, "")
        runID = c.or(.runID, "")
        runNumber = c.or(.runNumber, "")
        serverURL = c.or(.serverURL, "")
        startedAt = c.maybe(.startedAt)
        cpuTicks = c.or(.cpuTicks, 0)
        rss = c.or(.rss, 0)
        processes = c.or(.processes, 0)
        cpu = c.or(.cpu, 0)
    }

    /// elapsedSeconds is the age of the job. It is 0 when the start time is
    /// missing or later than the reading.
    public func elapsedSeconds(now: Date) -> Double {
        guard let startedAt else { return 0 }
        return max(0, now.timeIntervalSince(startedAt))
    }

    /// runURL points at the run page of the forge. It is nil when the agent
    /// reports no server, no repository, or no run id.
    public var runURL: URL? {
        var base = serverURL.trimmingCharacters(in: .whitespaces)
        while base.hasSuffix("/") { base.removeLast() }
        guard !base.isEmpty, !repository.isEmpty, !runID.isEmpty else { return nil }
        return URL(string: base + "/" + repository + "/actions/runs/" + runID)
    }

    /// sortedByElapsed puts the oldest job first, because a long job is
    /// the one that a reader looks for.
    public static func sortedByElapsed(_ jobs: [RunnerJob], now: Date) -> [RunnerJob] {
        jobs.sorted { $0.elapsedSeconds(now: now) > $1.elapsedSeconds(now: now) }
    }

    /// title names the job for the screen.
    public var title: String {
        if workflow.isEmpty { return job.isEmpty ? "job \(workerPID)" : job }
        return job.isEmpty ? workflow : workflow + " · " + job
    }
}

/// RunnerMath derives the readings that one sample cannot hold alone.
public enum RunnerMath {
    /// jobCPU reads the CPU of one job from the tick change between two
    /// samples. The agent counts ticks at 100 for each second of one core,
    /// so the change over the time gives a percent of one core. The result
    /// is nil when the app cannot compute it, and never a false zero.
    public static func jobCPU(pid: Int, previous: Sample?, current: Sample) -> Double? {
        guard let previous,
            let old = previous.runnerJobs.first(where: { $0.workerPID == pid }),
            let new = current.runnerJobs.first(where: { $0.workerPID == pid })
        else { return nil }
        let seconds = current.at.timeIntervalSince(previous.at)
        guard seconds > 0, new.cpuTicks >= old.cpuTicks else { return nil }
        return Double(new.cpuTicks - old.cpuTicks) / seconds
    }
}
