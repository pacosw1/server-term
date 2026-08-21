import SwiftUI
import ServtermKit

/// MeterView is the capacity bar. It draws a bar only for a known reading,
/// because an empty bar reads as "plenty left" instead of "unknown".
struct MeterView: View {
    let label: String
    let percent: Double?
    var detail: String = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                Text(label)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                Spacer(minLength: 8)
                Text(Format.optionalPercent(percent))
                    .font(.subheadline)
                    .bold()
                    .monospacedDigit()
                    .contentTransition(.numericText())
                    .foregroundStyle(Theme.color(for: percent))
            }
            if let percent {
                MeterBar(percent: percent)
            }
            if !detail.isEmpty {
                Text(detail)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
        .animation(.easeOut(duration: 0.35), value: percent)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label)
        .accessibilityValue(percent == nil ? "unknown" : Format.percent(percent!) + " " + detail)
    }
}

/// MeterBar draws the rounded track and its fill. It reads its own width,
/// because the fill must match the width of the card that holds it.
private struct MeterBar: View {
    let percent: Double

    var body: some View {
        GeometryReader { proxy in
            let fraction = min(max(percent / 100, 0), 1)
            ZStack(alignment: .leading) {
                Capsule().fill(.quaternary)
                Capsule()
                    .fill(Theme.gradient(for: percent))
                    .frame(width: max(proxy.size.width * fraction, fraction > 0 ? 6 : 0))
            }
        }
        .frame(height: Theme.meterHeight)
    }
}
