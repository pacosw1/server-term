import SwiftUI
import ServtermKit

/// ServerCardView is one server row: a status dot, the name, the key
/// numbers, and the CPU trend of the last minutes.
struct ServerCardView: View {
    let server: ServerEntry
    let reading: Reading<Sample>?
    let trend: [MetricPoint]
    let transport: Transport

    private var sample: Sample? { reading?.value }
    private var isStale: Bool { reading?.error != nil && reading?.value != nil }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                Rectangle()
                    .fill(state.color)
                    .frame(width: 10, height: 10)
                    .padding(.top, 6)
                VStack(alignment: .leading, spacing: 2) {
                    Text(server.name)
                        .font(.headline)
                    if !server.location.isEmpty {
                        Text(server.location)
                            .font(.subheadline)
                            .foregroundStyle(Theme.muted)
                    }
                }
                Spacer(minLength: 8)
                VStack(alignment: .trailing, spacing: 6) {
                    StateChip(text: state.text, color: state.color, systemImage: state.icon)
                    TransportChip(transport: transport)
                }
            }
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "CPU", value: Format.optionalPercent(sample.map(\.cpuPercent)),
                    tint: Theme.color(for: sample?.cpuPercent))
                StatTile(
                    label: "Memory", value: Format.optionalPercent(sample?.memoryPercent),
                    tint: Theme.color(for: sample?.memoryPercent))
                StatTile(
                    label: "Disk", value: Format.optionalPercent(sample?.primaryDisk?.usedPercent),
                    tint: Theme.color(for: sample?.primaryDisk?.usedPercent))
            }
            BlockTrendView(
                columns: SparkBars.columns(points: trend, window: SparkBars.window),
                label: "CPU", height: 40, columnWidth: 10)
            HStack {
                AgeNote(fetchedAt: reading?.fetchedAt, isStale: isStale)
                Spacer(minLength: 8)
                Image(systemName: "chevron.right")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            if case .polling(let reason) = transport, reading?.error == nil {
                Text(reason)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if let error = reading?.error {
                ErrorBanner(message: error)
            }
        }
        .card()
    }

    private var state: (text: String, color: Color, icon: String) {
        if reading?.error != nil { return ("error", Theme.critical, "exclamationmark.triangle.fill") }
        guard let sample else {
            return reading?.isLoading == true
                ? ("reading", .secondary, "arrow.clockwise") : ("unknown", .secondary, "questionmark")
        }
        return sample.online
            ? ("online", Theme.normal, "checkmark.circle.fill")
            : ("offline", Theme.critical, "xmark.circle.fill")
    }
}
