import SwiftUI
import ServtermKit

struct ServerNetworkCard: View {
    let sample: Sample

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Network").font(.headline)
            LabeledContent("Link", value: value(sample.networkInterface))
            LabeledContent("Type", value: value(sample.networkType))
            LabeledContent(
                "Speed",
                value: sample.networkLinkMbps > 0 ? "\(sample.networkLinkMbps) Mbps" : Format.unknown)
            LabeledContent("Receive", value: Format.rate(bytesPerSecond: sample.netRxRate))
            LabeledContent("Send", value: Format.rate(bytesPerSecond: sample.netTxRate))
            LabeledContent("Received", value: Format.bytes(unsigned: sample.netRx))
            LabeledContent("Sent", value: Format.bytes(unsigned: sample.netTx))
            Divider()
            LabeledContent("Errors in / out", value: "\(sample.netRxErrors) / \(sample.netTxErrors)")
            LabeledContent("Drops in / out", value: "\(sample.netRxDrops) / \(sample.netTxDrops)")
            if sample.hasNetworkFaults {
                Label("The interface counted errors or dropped packets.", systemImage: "exclamationmark.triangle")
                    .font(.caption)
                    .foregroundStyle(Theme.warning)
            }
            Divider()
            LabeledContent(
                "Swap",
                value: "\(Format.bytes(unsigned: sample.swapUsedBytes)) of \(Format.bytes(unsigned: sample.swapTotal))")
            LabeledContent("Agent latency", value: latencyText)
        }
        .font(.subheadline)
        .card()
    }

    private var latencyText: String {
        guard let seconds = sample.latencySeconds else { return Format.unknown }
        return String(format: "%.0f ms", seconds * 1000)
    }

    private func value(_ text: String) -> String {
        text.isEmpty ? Format.unknown : text
    }
}
