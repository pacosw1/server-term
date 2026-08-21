import Charts
import SwiftUI
import ServtermKit

/// MetricChartView draws one history window. It shows an honest empty
/// state when the agent has no history for the window.
struct MetricChartView: View {
    let title: String
    let points: [MetricPoint]
    let tint: Color
    /// unit is "percent" for a share, or "rate" for bytes for each second.
    var isPercent: Bool = true
    var multiSeries: Bool = false
    /// window names the time span, because the chart hides its time axis:
    /// a label at the edge of a narrow card gets cut in half.
    var window: String = "last 10 minutes"

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(title)
                        .font(.headline)
                    Text(window)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer(minLength: 8)
                Text(latestText)
                    .font(.subheadline)
                    .monospacedDigit()
                    .contentTransition(.numericText())
                    .foregroundStyle(.secondary)
            }
            if points.count < 2 {
                Text("The agent has no history for this window yet.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .frame(height: Theme.chartHeight / 2, alignment: .center)
            } else {
                chart
            }
        }
        .card()
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(title)
        .accessibilityValue(latestText)
    }

    private var chart: some View {
        Chart(points) { point in
            AreaMark(
                x: .value("time", point.at),
                y: .value(title, point.value),
                series: .value("series", point.name))
                .interpolationMethod(.monotone)
                .foregroundStyle(Theme.seriesGradient(color(for: point.name)))
            LineMark(
                x: .value("time", point.at),
                y: .value(title, point.value),
                series: .value("series", point.name))
                .interpolationMethod(.monotone)
                .lineStyle(StrokeStyle(lineWidth: 2, lineCap: .round))
                .foregroundStyle(color(for: point.name))
        }
        .modifier(PercentScale(isPercent: isPercent))
        .chartXAxis {
            AxisMarks(values: .automatic(desiredCount: 4)) {
                AxisGridLine().foregroundStyle(.quaternary)
            }
        }
        .chartYAxis {
            AxisMarks(values: .automatic(desiredCount: 4)) { value in
                AxisGridLine().foregroundStyle(.quaternary)
                AxisValueLabel {
                    if let number = value.as(Double.self) {
                        Text(isPercent ? "\(Int(number))%" : Format.rate(bytesPerSecond: number))
                    }
                }
            }
        }
        .chartLegend(multiSeries ? .visible : .hidden)
        .frame(height: Theme.chartHeight)
        // A live feed adds one point for each second. The move is animated
        // so the chart slides instead of jumping.
        .animation(.linear(duration: 0.5), value: points.last?.at)
    }

    private func color(for name: String) -> Color {
        guard multiSeries else { return tint }
        return name == "send" ? Theme.violet : tint
    }

    private var latestText: String {
        guard let last = points.last else { return Format.unknown }
        if multiSeries {
            let receive = points.last { $0.name == "receive" }?.value ?? 0
            let send = points.last { $0.name == "send" }?.value ?? 0
            return Format.rate(bytesPerSecond: receive) + " in · " + Format.rate(bytesPerSecond: send) + " out"
        }
        return isPercent ? Format.percent(last.value) : Format.rate(bytesPerSecond: last.value)
    }
}

/// PercentScale pins a percent chart to the full scale, so a quiet minute
/// does not look like a busy one. A rate chart keeps the automatic scale.
private struct PercentScale: ViewModifier {
    let isPercent: Bool

    func body(content: Content) -> some View {
        if isPercent {
            content.chartYScale(domain: 0...100)
        } else {
            content
        }
    }
}
