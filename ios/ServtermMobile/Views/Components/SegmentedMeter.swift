import SwiftUI
import ServtermKit

/// SegmentedMeter draws the counted cells of the terminal meter. Every cell
/// is a square block with a small gap. The unfilled cells stay visible, so
/// the eye reads the whole scale, and the bar never looks continuous.
struct SegmentedMeter: View {
    let percent: Double
    var cells: Int = BarMeter.wideCells
    var height: CGFloat = 12
    var gap: CGFloat = 2

    private var filled: Int { BarMeter.filledCells(percent: percent, cells: cells) }

    var body: some View {
        HStack(spacing: gap) {
            ForEach(0..<cells, id: \.self) { index in
                Rectangle()
                    .fill(index < filled ? Theme.fillColor(for: percent) : Theme.dimTrack)
                    .frame(maxWidth: .infinity)
            }
        }
        .frame(height: height)
        .accessibilityHidden(true)
    }
}
