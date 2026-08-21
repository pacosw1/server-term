import SwiftUI
import ServtermKit

/// CoreGridView reads like the terminal: the core index with a leading
/// zero, a seven cell bar, then the percent with one decimal. The columns
/// stay aligned because every figure uses monospaced digits.
struct CoreGridView: View {
    let cores: [Double]

    private let columns = [GridItem(.adaptive(minimum: 148), spacing: 12)]

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Logical CPU usage").font(.headline)
            if cores.isEmpty {
                Text("This host reports no reading for each core.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            } else {
                LazyVGrid(columns: columns, alignment: .leading, spacing: 8) {
                    ForEach(Array(cores.enumerated()), id: \.offset) { index, value in
                        CoreRow(index: index, percent: value)
                    }
                }
            }
        }
        .card()
    }
}

/// CoreRow is one core: the name, the seven cell bar and the percent.
struct CoreRow: View {
    let index: Int
    let percent: Double

    var body: some View {
        HStack(spacing: 6) {
            Text(String(format: "CPU%02d", index))
                .font(.caption)
                .monospaced()
                .foregroundStyle(Theme.muted)
            SegmentedMeter(percent: percent, cells: BarMeter.coreCells, height: 10, gap: 1.5)
                .frame(width: 46)
            Text(String(format: "%5.1f%%", percent))
                .font(.caption)
                .monospacedDigit()
                .foregroundStyle(Theme.color(for: percent))
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("core \(index)")
        .accessibilityValue(Format.percent(percent))
    }
}
