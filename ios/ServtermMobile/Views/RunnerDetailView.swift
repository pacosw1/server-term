import SwiftUI
import ServtermKit

/// RunnerDetailView shows the runners of one server: the summary, the
/// share of the machine, and every job that runs on it now.
struct RunnerDetailView: View {
    @Environment(AppModel.self) private var model
    let server: ServerEntry

    private var reading: Reading<Sample>? { model.servers[server.id] }

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    if let error = reading?.error {
                        ErrorBanner(message: error)
                    }
                    if let sample = reading?.value {
                        summary(sample)
                        jobs(sample)
                    } else {
                        Text("The app has no reading of this server yet.")
                            .foregroundStyle(Theme.muted)
                            .card()
                    }
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
            .refreshable { await model.refreshAllServers(force: true) }
        }
        .navigationTitle(server.name)
        .navigationBarTitleDisplayMode(.inline)
        .onAppear { model.setLiveWants([server.id], for: "runner-detail") }
        .onDisappear { model.setLiveWants([], for: "runner-detail") }
        .pollEvery(seconds: 3) { await model.refresh(server: server) }
    }

    private func summary(_ sample: Sample) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Runners").font(.headline)
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "Listeners", value: "\(sample.runners.listeners)",
                    systemImage: "antenna.radiowaves.left.and.right")
                StatTile(
                    label: "Active", value: "\(sample.runners.activeJobs)",
                    tint: sample.runners.activeJobs > 0 ? Theme.normal : Theme.text,
                    systemImage: "bolt.fill")
                StatTile(
                    label: "Processes", value: "\(sample.runners.processes)",
                    systemImage: "square.stack.3d.up")
            }
            MeterView(
                label: "Share of the machine",
                percent: sample.runners.hostShare(cores: sample.cores),
                detail: "\(Format.cores(cpuPercent: sample.runners.cpu)) of \(sample.cores) cores")
            InfoRow("Runner CPU", value: Format.percent(sample.runners.cpu))
            InfoRow("Runner memory", value: Format.bytes(unsigned: sample.runners.rss))
            InfoRow("Memory share", value: Format.percent(sample.runners.memory))
            HStack {
                AgeNote(fetchedAt: reading?.fetchedAt, isStale: reading?.error != nil)
                Spacer(minLength: 8)
                TransportChip(transport: model.transports[server.id] ?? .idle)
            }
        }
        .font(.subheadline)
        .card()
    }

    private func jobs(_ sample: Sample) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Jobs").font(.headline)
            if sample.runnerJobs.isEmpty {
                Text("No job runs on this server now.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(RunnerJob.sortedByElapsed(sample.runnerJobs, now: sample.at)) { job in
                NavigationLink(value: JobRoute(server: server, job: job)) {
                    RunnerJobRow(
                        job: job, now: sample.at,
                        cpu: model.jobCPU(pid: job.workerPID, serverID: server.id))
                }
                .buttonStyle(.plain)
            }
        }
        .card()
    }
}
