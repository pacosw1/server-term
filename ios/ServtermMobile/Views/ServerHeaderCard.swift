import SwiftUI
import ServtermKit

/// ServerHeaderCard holds the machine facts. The headline numbers sit above
/// it on the detail screen, so this card carries only the details.
struct ServerHeaderCard: View {
    let sample: Sample

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            InfoRow("Host", value: text(sample.hostname))
            InfoRow("System", value: text(sample.os))
            InfoRow("Kernel", value: text(sample.kernel))
            InfoRow("Cores", value: sample.cores > 0 ? "\(sample.cores)" : Format.unknown)
            InfoRow("Uptime", value: Format.duration(seconds: sample.uptimeSeconds))
            InfoRow(
                "Load 1 / 5 / 15",
                value: String(format: "%.2f  %.2f  %.2f", sample.load1, sample.load5, sample.load15))
            if let power = sample.power {
                InfoRow("Power", value: String(format: "%.1f W", power))
            }
            if let battery = sample.batteryLevel {
                InfoRow(
                    "Battery",
                    value: Format.percent(battery) + (sample.batteryCharging ? " charging" : ""))
            }
        }
        .font(.subheadline)
        .card()
    }

    private func text(_ value: String) -> String {
        value.isEmpty ? Format.unknown : value
    }
}
