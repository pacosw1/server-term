import SwiftUI
import ServtermKit

/// BlockTrendView draws the trend as discrete columns, the way the
/// terminal draws its spark. Every column takes one of eight steps, the
/// tops are square, and a baseline holds the row. A reading that the agent
/// could not take draws a low mark, never a gap and never a zero column.
struct BlockTrendView: View {
    let columns: [SparkColumn]
    let label: String
    var height: CGFloat = 44
    var gap: CGFloat = 2
    /// columnWidth fixes the width of one column. A trend that is still
    /// filling then keeps its cell size instead of stretching a few wide
    /// blocks across the whole card.
    var columnWidth: CGFloat?

    var body: some View {
        VStack(spacing: 2) {
            HStack(alignment: .bottom, spacing: gap) {
                if columns.isEmpty {
                    Text("no trend yet")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .frame(maxWidth: .infinity, alignment: .center)
                } else {
                    ForEach(columns) { column in
                        Rectangle()
                            .fill(color(for: column))
                            .frame(
                                width: columnWidth,
                                height: height * fraction(of: column),
                                alignment: .bottom)
                            .frame(maxWidth: columnWidth == nil ? .infinity : columnWidth)
                    }
                    if columnWidth != nil {
                        Spacer(minLength: 0)
                    }
                }
            }
            .frame(height: height, alignment: .bottom)
            Rectangle()
                .fill(Theme.border)
                .frame(height: 1)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label + " trend")
        .accessibilityValue(spokenValue)
    }

    private func fraction(of column: SparkColumn) -> CGFloat {
        CGFloat(column.step) / CGFloat(SparkBars.steps)
    }

    private func color(for column: SparkColumn) -> Color {
        column.isOffline ? Theme.muted : Theme.fillColor(for: column.percent)
    }

    /// A screen reader hears the newest reading and the shape in words, not
    /// a row of blocks.
    private var spokenValue: String {
        guard let last = columns.last else { return "no reading" }
        if last.isOffline { return "the last reading is offline" }
        let readings = columns.compactMap(\.percent)
        guard let low = readings.min(), let high = readings.max() else { return "no reading" }
        return "now \(Format.percent(last.percent ?? 0)), lowest \(Format.percent(low)), highest \(Format.percent(high))"
    }
}
