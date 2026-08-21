import Foundation
import Testing
@testable import ServtermKit

/// The runner keys come from the real agent answer. Only worker_pid has a
/// JSON tag; every other key is the Go field name.
private let runnerBody = """
[{"version":1,"node_id":"node-a","sample":{
  "At":"2026-08-21T04:48:00Z","Online":true,"Hostname":"node-a","Cores":32,
  "Runners":{"Listeners":16,"ActiveJobs":5,"CPU":503.76,"Memory":17.8,
             "RSS":14107850752,"Processes":88,"CPUTicks":62260},
  "RunnerJobs":[{"worker_pid":995667,"Runner":"ci-1","Repository":"owner/repo",
                 "Workflow":"CI","Job":"e2e","RunID":"32445634823","RunNumber":"1246",
                 "ServerURL":"https://github.com","StartedAt":"2026-08-21T04:47:54.71Z",
                 "CPUTicks":2296,"RSS":4006518784,"Processes":33,"CPU":0}]}},
 {"version":1,"node_id":"node-a","sample":{
  "At":"2026-08-21T04:48:02Z","Online":true,"Hostname":"node-a","Cores":32,
  "Runners":{"Listeners":16,"ActiveJobs":5,"CPU":510.0,"Memory":18.0,
             "RSS":14107850752,"Processes":88,"CPUTicks":62500},
  "RunnerJobs":[{"worker_pid":995667,"Runner":"ci-1","Repository":"owner/repo",
                 "Workflow":"CI","Job":"e2e","RunID":"32445634823","RunNumber":"1246",
                 "ServerURL":"https://github.com","StartedAt":"2026-08-21T04:47:54.71Z",
                 "CPUTicks":2696,"RSS":4006518784,"Processes":33,"CPU":0}]}}]
"""

@Suite("Runners")
struct RunnerTests {
    private func page() throws -> [Sample] {
        try JSONDecoding.agent.decode([WireSample].self, from: Data(runnerBody.utf8)).map(\.sample)
    }

    @Test("the runner summary decodes")
    func runnerStats() throws {
        let sample = try page()[0]
        #expect(sample.runners.listeners == 16)
        #expect(sample.runners.activeJobs == 5)
        #expect(abs(sample.runners.cpu - 503.76) < 0.01)
        #expect(sample.runners.rss == 14_107_850_752)
        #expect(sample.runners.processes == 88)
    }

    @Test("a sample without runners reports zero listeners and no job")
    func noRunners() {
        #expect(Sample.empty.runners.listeners == 0)
        #expect(Sample.empty.runnerJobs.isEmpty)
        #expect(Sample.empty.hasRunners == false)
    }

    @Test("a listener alone counts as a runner host")
    func hasRunners() throws {
        #expect(try page()[0].hasRunners)
    }

    @Test("one job decodes every field that the screen shows")
    func runnerJob() throws {
        let job = try #require(page()[0].runnerJobs.first)
        #expect(job.workerPID == 995667)
        #expect(job.runner == "ci-1")
        #expect(job.repository == "owner/repo")
        #expect(job.workflow == "CI")
        #expect(job.job == "e2e")
        #expect(job.runNumber == "1246")
        #expect(job.rss == 4_006_518_784)
        #expect(job.processes == 33)
        #expect(job.runURL?.absoluteString == "https://github.com/owner/repo/actions/runs/32445634823")
    }

    @Test("the elapsed time comes from the start time and the sample time")
    func elapsed() throws {
        let sample = try page()[0]
        let job = try #require(sample.runnerJobs.first)
        #expect(abs(job.elapsedSeconds(now: sample.at) - 5.29) < 0.01)
    }

    @Test("the elapsed time is never negative")
    func elapsedFloor() throws {
        let job = try #require(page()[0].runnerJobs.first)
        #expect(job.elapsedSeconds(now: Date(timeIntervalSince1970: 0)) == 0)
    }

    @Test("the job CPU comes from the tick change between two samples")
    func derivedJobCPU() throws {
        let samples = try page()
        let cpu = try #require(RunnerMath.jobCPU(pid: 995667, previous: samples[0], current: samples[1]))
        // 400 ticks in 2 seconds is 200 percent of one core.
        #expect(abs(cpu - 200) < 0.001)
    }

    @Test("the job CPU is unknown when no earlier sample holds the job")
    func unknownJobCPU() throws {
        let samples = try page()
        #expect(RunnerMath.jobCPU(pid: 1, previous: samples[0], current: samples[1]) == nil)
        #expect(RunnerMath.jobCPU(pid: 995667, previous: nil, current: samples[1]) == nil)
    }

    @Test("a tick counter that goes backwards gives no reading")
    func restartedCounter() throws {
        let samples = try page()
        #expect(RunnerMath.jobCPU(pid: 995667, previous: samples[1], current: samples[0]) == nil)
    }

    @Test("the runner CPU converts to whole cores and to a host share")
    func cores() throws {
        let sample = try page()[0]
        #expect(Format.cores(cpuPercent: sample.runners.cpu) == "5.04 cores")
        #expect(Format.optionalPercent(sample.runners.hostShare(cores: sample.cores)).hasPrefix("15.7"))
        #expect(sample.runners.hostShare(cores: 0) == nil)
    }
}
