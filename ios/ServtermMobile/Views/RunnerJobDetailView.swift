import SwiftUI
import ServtermKit

/// RunnerJobDetailView shows everything that the agent reports about one
/// CI job, and the trend that the app collects while the screen is open.
struct RunnerJobDetailView: View {
    @Environment(AppModel.self) private var model
    let server: ServerEntry
    let job: RunnerJob

    /// live is the job as the newest reading holds it. The job that opened
    /// the screen is only the starting point.
    private var live: RunnerJob? {
        model.servers[server.id]?.value?.runnerJobs.first { $0.workerPID == job.workerPID }
    }

    private var now: Date { model.servers[server.id]?.value?.at ?? Date() }

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    if live == nil {
                        Text("This job is no longer in the reading. It finished, or the runner dropped it.")
                            .font(.footnote)
                            .foregroundStyle(Theme.warning)
                            .card()
                    }
                    identity
                    usage
                    trends
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle(current.job.isEmpty ? "job" : current.job)
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            model.setLiveWants([server.id], for: "job-detail")
            model.watchJob(pid: job.workerPID, serverID: server.id)
        }
        .onDisappear {
            model.setLiveWants([], for: "job-detail")
            model.stopWatchingJob(pid: job.workerPID, serverID: server.id)
        }
        .pollEvery(seconds: 3) { await model.refresh(server: server) }
    }

    private var current: RunnerJob { live ?? job }

    private var identity: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(current.repository.isEmpty ? "unknown repository" : current.repository)
                .font(.headline)
            InfoRow("Workflow", value: value(current.workflow))
            InfoRow("Job", value: value(current.job))
            InfoRow("Runner", value: value(current.runner))
            InfoRow("Run number", value: value(current.runNumber))
            InfoRow("Run id", value: value(current.runID))
            InfoRow("Worker process", value: current.workerPID > 0 ? "\(current.workerPID)" : Format.unknown)
            InfoRow("Started", value: startedText)
            InfoRow("Elapsed", value: Format.duration(seconds: current.elapsedSeconds(now: now)))
            if let url = current.runURL {
                Link("Open the run page", destination: url)
                    .font(.subheadline)
                    .foregroundStyle(Theme.accent)
                    .frame(minHeight: Theme.minimumTapTarget, alignment: .leading)
            }
        }
        .font(.subheadline)
        .card()
    }

    private var usage: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "CPU", value: Format.optionalPercent(cpu),
                    tint: cpu == nil ? Theme.muted : Theme.accent, systemImage: "cpu")
                StatTile(
                    label: "Memory", value: Format.bytes(unsigned: current.rss),
                    systemImage: "memorychip")
                StatTile(
                    label: "Processes", value: "\(current.processes)",
                    systemImage: "square.stack.3d.up")
            }
            Text("A job CPU above 100 percent means more than one core, so it carries no warning colour.")
                .font(.caption)
                .foregroundStyle(Theme.muted)
        }
        .card()
    }

    private var trends: some View {
        VStack(alignment: .leading, spacing: Theme.cardSpacing) {
            BlockChartCard(
                title: "Job CPU",
                columns: SparkBars.columns(
                    points: model.jobCPUTrend(pid: job.workerPID, serverID: server.id),
                    scale: .relative),
                latest: cpu,
                isPercent: true,
                window: "since this screen opened")
            BlockChartCard(
                title: "Job memory",
                columns: SparkBars.columns(
                    points: model.jobRSSTrend(pid: job.workerPID, serverID: server.id),
                    scale: .relative),
                latest: Double(current.rss),
                isPercent: false,
                window: "since this screen opened",
                latestOverride: Format.bytes(unsigned: current.rss))
        }
    }

    private var cpu: Double? {
        model.jobCPU(pid: job.workerPID, serverID: server.id)
    }

    private var startedText: String {
        guard let startedAt = current.startedAt else { return Format.unknown }
        return startedAt.formatted(date: .omitted, time: .standard)
    }

    private func value(_ text: String) -> String {
        text.isEmpty ? Format.unknown : text
    }
}
