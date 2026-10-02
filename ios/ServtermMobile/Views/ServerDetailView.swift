import SwiftUI
import ServtermKit

/// ServerDetailView puts the two things a user comes for at the top: the
/// terminal and the four headline numbers. Every other reading folds away
/// under "More details".
struct ServerDetailView: View {
    @Environment(AppModel.self) private var model
    @State private var showsMore = false
    let server: ServerEntry

    private var reading: Reading<Sample>? { model.servers[server.id] }
    private var history: [Sample] { model.histories[server.id] ?? [] }

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    NavigationLink(value: SessionsRoute(server: server)) {
                        Label("Open terminal", systemImage: "terminal")
                            .font(.headline)
                            .foregroundStyle(Theme.base)
                            .frame(maxWidth: .infinity, minHeight: 52)
                            .background(Theme.accent)
                            .clipShape(.rect(cornerRadius: Theme.cardRadius))
                    }
                    .buttonStyle(.plain)
                    .accessibilityIdentifier("open-shell")

                    if let sample = reading?.value {
                        if reading?.error != nil {
                            Label("Not updating. Showing the last reading.", systemImage: "clock.badge.exclamationmark")
                                .font(.footnote)
                                .foregroundStyle(Theme.warning)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        headline(sample)
                        moreToggle
                        if showsMore {
                            details(sample)
                        }
                    } else if let error = reading?.error {
                        ErrorBanner(message: error)
                    } else {
                        ProgressView()
                            .frame(maxWidth: .infinity, minHeight: 120)
                    }
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle(server.name)
        .navigationBarTitleDisplayMode(.inline)
        .navigationDestination(for: SessionsRoute.self) { route in
            SessionsView(server: route.server)
        }
        .navigationDestination(for: ShellRoute.self) { route in
            ShellView(server: route.server, session: route.session)
        }
        .task { await model.loadHistory(server: server) }
        .onAppear { model.setLiveWants([server.id], for: "detail") }
        .onDisappear { model.setLiveWants([], for: "detail") }
        .refreshable { await model.loadHistory(server: server) }
        .pollEvery(seconds: 3) {
            await model.refresh(server: server)
        }
    }

    private func headline(_ sample: Sample) -> some View {
        VStack(spacing: 16) {
            HStack(spacing: 12) {
                StatTile(
                    label: "CPU", value: Format.percent(sample.cpuPercent),
                    tint: Theme.color(for: sample.cpuPercent))
                StatTile(
                    label: "Memory", value: Format.optionalPercent(sample.memoryPercent),
                    tint: Theme.color(for: sample.memoryPercent))
            }
            HStack(spacing: 12) {
                StatTile(
                    label: "Disk", value: Format.optionalPercent(sample.primaryDisk?.usedPercent),
                    tint: Theme.color(for: sample.primaryDisk?.usedPercent))
                StatTile(label: "Uptime", value: Format.duration(seconds: sample.uptimeSeconds))
            }
        }
        .card()
    }

    private var moreToggle: some View {
        Button {
            withAnimation(.snappy) { showsMore.toggle() }
        } label: {
            HStack {
                Text(showsMore ? "Fewer details" : "More details")
                Spacer()
                Image(systemName: "chevron.down")
                    .rotationEffect(.degrees(showsMore ? 180 : 0))
            }
            .font(.subheadline)
            .foregroundStyle(Theme.muted)
            .frame(minHeight: Theme.minimumTapTarget)
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("more-details")
    }

    @ViewBuilder
    private func details(_ sample: Sample) -> some View {
        BlockChartCard(
            title: "CPU",
            columns: SparkBars.columns(from: history, window: 36, mode: .spread) { $0.cpuPercent },
            latest: sample.cpuPercent,
            isPercent: true)
        BlockChartCard(
            title: "Memory",
            columns: SparkBars.columns(from: history, window: 36, mode: .spread) { $0.memoryPercent ?? 0 },
            latest: sample.memoryPercent,
            isPercent: true)
        ServerProcessCard(sample: sample)
        ServerStorageCard(disks: sample.sortedDisks)
        ServerNetworkCard(sample: sample)
        // A card appears only when the host reports that kind, so a
        // virtual machine with no sensor shows no empty block.
        if sample.hasSensors {
            TemperaturesCard(temperatures: sample.temperatures)
        }
        if !sample.accelerators.isEmpty {
            ServerAcceleratorCard(accelerators: sample.accelerators)
        }
        ServerHeaderCard(sample: sample)
    }
}
