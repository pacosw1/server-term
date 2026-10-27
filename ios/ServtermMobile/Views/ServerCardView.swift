import SwiftUI
import ServtermKit

/// ServerCardView is one server row: a status dot, the name, and three
/// numbers. Everything else lives one tap deeper.
struct ServerCardView: View {
    let server: ServerEntry
    let reading: Reading<Sample>?
    /// hasToken is false for a server that the app found but cannot read
    /// yet. It says so, rather than look like a server that is down.
    var hasToken = true

    private var sample: Sample? { reading?.value }

    var body: some View {
        HStack(spacing: 12) {
            Circle()
                .fill(status.color)
                .frame(width: 10, height: 10)
                .accessibilityLabel(status.text)
            VStack(alignment: .leading, spacing: 2) {
                Text(server.name)
                    .font(.headline)
                    .lineLimit(1)
                if let note = status.note {
                    Text(note)
                        .font(.caption)
                        .foregroundStyle(status.color)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 8)
            if let sample, sample.online {
                HStack(spacing: 14) {
                    Metric(label: "CPU", percent: sample.cpuPercent)
                    Metric(label: "MEM", percent: sample.memoryPercent)
                    Metric(label: "DISK", percent: sample.primaryDisk?.usedPercent)
                }
            }
            Image(systemName: "chevron.right")
                .font(.footnote)
                .foregroundStyle(Theme.muted)
        }
        .frame(minHeight: Theme.minimumTapTarget)
        .card()
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("server-card")
    }

    /// status is the one fact the dot says. A note shows only when
    /// something is wrong, so a healthy list reads as quiet.
    private var status: (text: String, color: Color, note: String?) {
        if let sample {
            if !sample.online { return ("offline", Theme.critical, "Offline") }
            if reading?.error != nil { return ("stale", Theme.warning, "Not updating") }
            return ("online", Theme.normal, nil)
        }
        if !hasToken { return ("needs token", Theme.warning, "Needs token") }
        if reading?.error != nil { return ("unreachable", Theme.critical, "Can't connect") }
        return ("loading", Theme.muted, nil)
    }
}

/// Metric is one small number with a label under it, coloured by its grade.
private struct Metric: View {
    let label: String
    let percent: Double?

    var body: some View {
        VStack(spacing: 0) {
            // A row has room for a whole number only. The detail screen
            // shows the decimals.
            Text(percent.map { "\(Int($0.rounded()))%" } ?? Format.unknown)
                .font(.subheadline.monospacedDigit().bold())
                .foregroundStyle(Theme.color(for: percent))
            Text(label)
                .font(.caption2)
                .foregroundStyle(Theme.muted)
        }
        .lineLimit(1)
        // The numbers keep their width; the name truncates instead.
        .fixedSize()
    }
}
