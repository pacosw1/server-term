import SwiftUI
import ServtermKit

/// CoreGridView shows the load of every core. The agent reports one figure
/// for each core, so a busy core is visible next to an idle one.
struct CoreGridView: View {
    let cores: [Double]

    private let columns = [GridItem(.adaptive(minimum: 64), spacing: 10)]

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Cores").font(.headline)
            if cores.isEmpty {
                Text("The agent reports no reading for each core.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            } else {
                LazyVGrid(columns: columns, alignment: .leading, spacing: 10) {
                    ForEach(Array(cores.enumerated()), id: \.offset) { index, value in
                        VStack(alignment: .leading, spacing: 4) {
                            Text("#\(index)")
                                .font(.caption)
                                .foregroundStyle(Theme.muted)
                            Text("\(Int(value.rounded()))%")
                                .font(.subheadline)
                                .monospacedDigit()
                                .contentTransition(.numericText())
                                .foregroundStyle(Theme.color(for: value))
                            Capsule()
                                .fill(Theme.raised)
                                .frame(height: 4)
                                .overlay(alignment: .leading) {
                                    Capsule()
                                        .fill(Theme.color(for: value))
                                        .frame(width: 48 * min(max(value, 0), 100) / 100, height: 4)
                                }
                                .frame(width: 48, alignment: .leading)
                        }
                        .accessibilityElement(children: .combine)
                        .accessibilityLabel("core \(index)")
                        .accessibilityValue(Format.percent(value))
                    }
                }
            }
        }
        .animation(.easeOut(duration: 0.4), value: cores)
        .card()
    }
}
