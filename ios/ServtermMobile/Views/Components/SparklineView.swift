import Charts
import SwiftUI
import ServtermKit

/// SparklineView is the small trend line on a server card. It draws
/// nothing when the app holds fewer than two points, so a single reading
/// never looks like a flat history.
struct SparklineView: View {
    let points: [MetricPoint]
    let tint: Color
    let label: String

    var body: some View {
        Group {
            if points.count < 2 {
                Text("no trend yet")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
                    .frame(height: Theme.sparklineHeight, alignment: .center)
            } else {
                Chart(points) { point in
                    AreaMark(
                        x: .value("time", point.at),
                        y: .value(label, point.value))
                        .interpolationMethod(.monotone)
                        .foregroundStyle(Theme.seriesGradient(tint))
                    LineMark(
                        x: .value("time", point.at),
                        y: .value(label, point.value))
                        .interpolationMethod(.monotone)
                        .lineStyle(StrokeStyle(lineWidth: 2, lineCap: .round))
                        .foregroundStyle(tint)
                }
                .chartXAxis(.hidden)
                .chartYAxis(.hidden)
                .chartYScale(domain: MetricSeries.paddedDomain(points, padding: 5, upperLimit: 100))
                .chartLegend(.hidden)
                .frame(height: Theme.sparklineHeight)
                .padding(.vertical, 4)
                .background(.quaternary.opacity(0.25), in: .rect(cornerRadius: 10))
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label + " trend")
        .accessibilityValue(accessibilityValue)
    }

    private var accessibilityValue: String {
        guard let last = points.last else { return "no reading" }
        return "now " + Format.percent(last.value)
    }
}
