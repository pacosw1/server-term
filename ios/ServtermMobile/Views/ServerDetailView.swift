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
                    if let sample = reading?.value {
                        ServerHeaderCard(
                            sample: sample, fetchedAt: reading?.fetchedAt,
                            isStale: reading?.error != nil,
                            transport: model.transports[server.id] ?? .idle,
                            roundTrip: model.roundTrips[server.id])
                        MetricChartView(
                            title: "CPU",
                            points: MetricSeries.downsample(MetricSeries.cpu(from: history), to: 120),
                            tint: Theme.accent)
                        MetricChartView(
                            title: "Memory",
                            points: MetricSeries.downsample(MetricSeries.memory(from: history), to: 120),
                            tint: Theme.violet)
                        MetricChartView(
                            title: "Network",
                            points: MetricSeries.downsample(MetricSeries.network(from: history), to: 240),
                            tint: Theme.normal, isPercent: false, multiSeries: true)
                        CoreGridView(cores: sample.corePercent)
                        ServerStorageCard(disks: sample.sortedDisks)
                        if !sample.devices.isEmpty {
                            ServerDevicesCard(devices: sample.devices)
                        }
                        ServerNetworkCard(sample: sample)
                        if sample.hasPressure {
                            ServerPressureCard(sample: sample)
                        }
                        if !sample.accelerators.isEmpty {
                            ServerAcceleratorCard(accelerators: sample.accelerators)
                        }
                        ServerProcessCard(sample: sample)
                    } else {
                        Text("The app has no reading of this server yet.")
                            .foregroundStyle(.secondary)
                            .card()
                    }
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle(server.name)
        .navigationBarTitleDisplayMode(.inline)
        .task { await model.loadHistory(server: server) }
        .onAppear { model.setLiveWants([server.id], for: "detail") }
        .onDisappear { model.setLiveWants([], for: "detail") }
        .refreshable { await model.loadHistory(server: server) }
        .pollEvery(seconds: 3) {
            await model.refresh(server: server)
        }
    }
}
