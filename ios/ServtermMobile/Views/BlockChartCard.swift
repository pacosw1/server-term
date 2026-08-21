import SwiftUI
import ServtermKit

/// BlockChartCard holds one trend of the last ten minutes, drawn as the
/// discrete columns of the terminal. A rate series has no ceiling, so its
/// columns scale against the busiest column in the same window; the label
/// still shows the real figure.
struct BlockChartCard: View {
    let title: String
    let columns: [SparkColumn]
    let latest: Double?
    let isPercent: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.headline)
                    Text(isPercent ? "last 10 minutes" : "last 10 minutes, scaled to the busiest minute")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                }
                Spacer(minLength: 8)
                Text(latestText)
                    .font(.subheadline)
                    .bold()
                    .monospacedDigit()
                    .contentTransition(.numericText())
                    .foregroundStyle(isPercent ? Theme.color(for: latest) : Theme.text)
            }
            BlockTrendView(columns: columns, label: title, height: 74)
        }
        .card()
    }

    private var latestText: String {
        guard let latest else { return Format.unknown }
        return isPercent ? Format.percent(latest) : Format.rate(bytesPerSecond: latest)
    }
}
