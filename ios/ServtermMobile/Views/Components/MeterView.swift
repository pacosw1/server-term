import SwiftUI
import ServtermKit

/// MeterView is the capacity bar. It draws a bar only for a known reading,
/// because an empty bar reads as "plenty left" instead of "unknown".
struct MeterView: View {
    let label: String
    let percent: Double?
    var detail: String = ""
    /// cells is the width of the meter in counted blocks. The terminal
    /// uses 24 for a plan or a disk meter and 10 for a compact one.
    var cells: Int = BarMeter.wideCells

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
                SegmentedMeter(percent: percent, cells: cells, height: Theme.meterHeight)
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
