import SwiftUI
import ServtermKit

/// ServerHeaderCard is the cluster of the four headline numbers, with the
/// machine facts under it.
struct ServerHeaderCard: View {
    let sample: Sample
    let fetchedAt: Date?
    let isStale: Bool
    let transport: Transport
    /// roundTrip is the time of the last poll request. It is nil while a
    /// socket feeds the screen, because a socket makes no request.
    let roundTrip: TimeInterval?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "CPU", value: Format.percent(sample.cpuPercent),
                    tint: Theme.color(for: sample.cpuPercent), systemImage: "cpu")
                StatTile(
                    label: "Memory", value: Format.optionalPercent(sample.memoryPercent),
                    tint: Theme.color(for: sample.memoryPercent), systemImage: "memorychip")
            }
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "Disk", value: Format.optionalPercent(sample.primaryDisk?.usedPercent),
                    tint: Theme.color(for: sample.primaryDisk?.usedPercent),
                    systemImage: "internaldrive")
                StatTile(
                    label: "Load", value: String(format: "%.2f", sample.load1),
                    systemImage: "gauge.with.dots.needle.50percent")
            }
            Divider()
            LabeledContent("Host", value: text(sample.hostname))
            LabeledContent("System", value: text(sample.os))
            LabeledContent("Kernel", value: text(sample.kernel))
            LabeledContent("Cores", value: sample.cores > 0 ? "\(sample.cores)" : Format.unknown)
            LabeledContent("Uptime", value: Format.duration(seconds: sample.uptimeSeconds))
            LabeledContent(
                "Load 1 / 5 / 15",
                value: String(format: "%.2f  %.2f  %.2f", sample.load1, sample.load5, sample.load15))
            if let power = sample.power {
                LabeledContent("Power", value: String(format: "%.1f W", power))
            }
            if let battery = sample.batteryLevel {
                LabeledContent(
                    "Battery",
                    value: Format.percent(battery) + (sample.batteryCharging ? " charging" : ""))
            }
            HStack {
                AgeNote(fetchedAt: fetchedAt, isStale: isStale)
                Spacer(minLength: 8)
                TransportChip(transport: transport)
            }
            if case .polling(let reason) = transport {
                Text(reason)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .font(.subheadline)
        .card()
    }

    private func text(_ value: String) -> String {
        value.isEmpty ? Format.unknown : value
    }
}
