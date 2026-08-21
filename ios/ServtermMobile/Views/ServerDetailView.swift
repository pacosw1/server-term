import SwiftUI
import ServtermKit

struct ServerDetailView: View {
    @Environment(AppModel.self) private var model
    let server: ServerEntry

    private var reading: Reading<Sample>? { model.servers[server.id] }
    private var history: [Sample] { model.histories[server.id] ?? [] }

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    if let error = reading?.error {
                        ErrorBanner(message: error)
                    }
                    NavigationLink(value: SessionsRoute(server: server)) {
                        Label("Shell sessions", systemImage: "terminal")
                            .font(.subheadline)
                            .foregroundStyle(Theme.accent)
                            .frame(maxWidth: .infinity, minHeight: Theme.minimumTapTarget, alignment: .leading)
                            .card()
                    }
                    .buttonStyle(.plain)
                    .accessibilityIdentifier("open-shell")
                    if let sample = reading?.value {
                        ServerHeaderCard(
                            sample: sample, fetchedAt: reading?.fetchedAt,
                            isStale: reading?.error != nil,
                            transport: model.transports[server.id] ?? .idle,
                            roundTrip: model.roundTrips[server.id])
                        BlockChartCard(
                            title: "CPU",
                            columns: SparkBars.columns(
                                from: history, window: 36, mode: .spread) { $0.cpuPercent },
                            latest: sample.cpuPercent,
                            isPercent: true)
                        BlockChartCard(
                            title: "Memory",
                            columns: SparkBars.columns(
                                from: history, window: 36, mode: .spread) { $0.memoryPercent ?? 0 },
                            latest: sample.memoryPercent,
                            isPercent: true)
                        BlockChartCard(
                            title: "Network in",
                            columns: SparkBars.columns(
                                from: history, window: 36, scale: .relative, mode: .spread) { $0.netRxRate },
                            latest: sample.netRxRate,
                            isPercent: false)
                        BlockChartCard(
                            title: "Network out",
                            columns: SparkBars.columns(
                                from: history, window: 36, scale: .relative, mode: .spread) { $0.netTxRate },
                            latest: sample.netTxRate,
                            isPercent: false)
                        CoreGridView(cores: sample.corePercent)
                        ServerStorageCard(disks: sample.sortedDisks)
                        if !sample.devices.isEmpty {
                            ServerDevicesCard(devices: sample.devices)
                        }
                        ServerNetworkCard(sample: sample)
                        if !sample.interfaces.isEmpty {
                            InterfacesCard(
                                interfaces: sample.interfaces,
                                ratesKnown: model.hasTwoReadings(server.id))
                        }
                        // A card appears only when the host reports that
                        // kind. A virtual machine has no sensor and macOS
                        // reports no block device traffic; that is normal,
                        // so the screen shows nothing instead of an empty
                        // block that reads as broken.
                        if sample.hasDiskIO {
                            DiskIOCard(
                                entries: sample.diskIO, disks: sample.disks,
                                ratesKnown: model.hasTwoReadings(server.id))
                        }
                        if sample.hasSensors {
                            TemperaturesCard(temperatures: sample.temperatures)
                        }
                        if sample.hasPressure {
                            ServerPressureCard(sample: sample)
                        }
                        if !sample.accelerators.isEmpty {
                            ServerAcceleratorCard(accelerators: sample.accelerators)
                        }
                        ServerProcessCard(sample: sample)
                    } else {
                        Text("The app has no reading of this server yet.")
                            .foregroundStyle(Theme.muted)
                            .card()
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
}
