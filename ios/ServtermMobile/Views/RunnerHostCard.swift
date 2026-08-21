import SwiftUI
import ServtermKit

/// RunnerHostCard shows the runner summary of one server and every job
/// that runs on it now.
struct RunnerHostCard: View {
    @Environment(AppModel.self) private var model
    let server: ServerEntry
    let reading: Reading<Sample>?

    private var sample: Sample? { reading?.value }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            NavigationLink(value: RunnerRoute(server: server)) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(server.name).font(.headline)
                        if !server.location.isEmpty {
                            Text(server.location).font(.subheadline).foregroundStyle(Theme.muted)
                        }
                    }
                    Spacer(minLength: 8)
                    StateChip(
                        text: "\(sample?.runners.activeJobs ?? 0) active",
                        color: (sample?.runners.activeJobs ?? 0) > 0 ? Theme.normal : Theme.muted,
                        systemImage: "bolt.fill")
                    Image(systemName: "chevron.right")
                        .font(.footnote)
                        .foregroundStyle(Theme.muted)
                }
                .frame(minHeight: Theme.minimumTapTarget)
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("runner-header")
            if let sample {
                HStack(alignment: .top, spacing: 12) {
                    StatTile(
                        label: "Listeners", value: "\(sample.runners.listeners)",
                        systemImage: "antenna.radiowaves.left.and.right")
                    StatTile(
                        label: "CPU", value: Format.cores(cpuPercent: sample.runners.cpu),
                        tint: Theme.color(for: sample.runners.hostShare(cores: sample.cores)),
                        systemImage: "cpu")
                    StatTile(
                        label: "Memory", value: Format.bytes(unsigned: sample.runners.rss),
                        systemImage: "memorychip")
                }
                MeterView(
                    label: "Share of the machine",
                    percent: sample.runners.hostShare(cores: sample.cores),
                    detail: "\(sample.runners.processes) processes on \(sample.cores) cores")
                if sample.runnerJobs.isEmpty {
                    Text("No job runs now.")
                        .font(.footnote)
                        .foregroundStyle(Theme.muted)
                } else {
                    ForEach(RunnerJob.sortedByElapsed(sample.runnerJobs, now: sample.at)) { job in
                        NavigationLink(value: JobRoute(server: server, job: job)) {
                            RunnerJobRow(
                                job: job, now: sample.at,
                                cpu: model.jobCPU(pid: job.workerPID, serverID: server.id))
                                .contentShape(.rect)
                        }
                        .buttonStyle(.plain)
                        .accessibilityIdentifier("job-row")
                    }
                }
                AgeNote(fetchedAt: reading?.fetchedAt, isStale: reading?.error != nil)
            }
            if let error = reading?.error {
                ErrorBanner(message: error)
            }
        }
        .card()
    }
}
