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
                    .foregroundStyle(Theme.muted)
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
                    .foregroundStyle(Theme.muted)
            }
        }
        .animation(.easeOut(duration: 0.35), value: percent)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label)
        .accessibilityValue(percent == nil ? "unknown" : Format.percent(percent!) + " " + detail)
    }
}

/// MeterBar draws a square track with a hard border and a solid fill. It
/// reads its own width, because the fill must match the card that holds it.
private struct MeterBar: View {
    let percent: Double

    var body: some View {
        GeometryReader { proxy in
            let fraction = min(max(percent / 100, 0), 1)
            ZStack(alignment: .leading) {
                Rectangle().fill(Theme.base)
                Rectangle()
                    .fill(Theme.meterFill(percent))
                    .frame(width: max(proxy.size.width * fraction, fraction > 0 ? 4 : 0))
            }
            .overlay {
                Rectangle().strokeBorder(Theme.border, lineWidth: 1)
            }
        }
        .frame(height: Theme.meterHeight)
    }
}
