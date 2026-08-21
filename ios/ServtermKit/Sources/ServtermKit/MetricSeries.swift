import Foundation

/// MetricPoint is one point of a chart. The name tells two series apart in
/// one chart, for example the receive rate and the send rate.
public struct MetricPoint: Sendable, Equatable, Identifiable {
    public let at: Date
    public let value: Double
    public let name: String

    public init(at: Date, value: Double, name: String = "") {
        self.at = at
        self.value = value
        self.name = name
    }

    public var id: String { "\(name)-\(at.timeIntervalSince1970)" }
}

/// MetricSeries shapes a history page into chart points. It leaves out
/// every sample that holds no reading, so a chart never draws a false zero.
public enum MetricSeries {
    public static func cpu(from samples: [Sample]) -> [MetricPoint] {
        samples
            .filter(\.online)
            .sorted { $0.at < $1.at }
            .map { MetricPoint(at: $0.at, value: $0.cpuPercent, name: "cpu") }
    }

    public static func memory(from samples: [Sample]) -> [MetricPoint] {
        samples
            .filter(\.online)
            .sorted { $0.at < $1.at }
            .compactMap { sample in
                guard let percent = sample.memoryPercent else { return nil }
                return MetricPoint(at: sample.at, value: percent, name: "memory")
            }
    }

    /// network returns the two rates in one list, so one chart can draw
    /// both series.
    public static func network(from samples: [Sample]) -> [MetricPoint] {
        samples
            .filter(\.online)
            .sorted { $0.at < $1.at }
            .flatMap {
                [
                    MetricPoint(at: $0.at, value: $0.netRxRate, name: "receive"),
                    MetricPoint(at: $0.at, value: $0.netTxRate, name: "send"),
                ]
            }
    }

    /// runnerCPU is the CPU that every runner on the server uses together.
    public static func runnerCPU(from samples: [Sample]) -> [MetricPoint] {
        samples
            .filter(\.online)
            .sorted { $0.at < $1.at }
            .map { MetricPoint(at: $0.at, value: $0.runners.cpu, name: "runners") }
    }

    /// downsample thins a long series so a chart draws fewer marks. It
    /// keeps the first point and the last point, so the window does not
    /// move when the chart redraws.
    public static func downsample(_ points: [MetricPoint], to limit: Int) -> [MetricPoint] {
        guard limit > 1, points.count > limit else { return points }
        let step = Double(points.count - 1) / Double(limit - 1)
        var thin: [MetricPoint] = []
        thin.reserveCapacity(limit)
        for index in 0..<limit {
            thin.append(points[Int((Double(index) * step).rounded())])
        }
        if thin.last != points.last, let last = points.last { thin[thin.count - 1] = last }
        return thin
    }

    /// paddedDomain frames a small chart around its own data, so a quiet
    /// series still shows its shape. It never leaves the allowed range and
    /// it always keeps some height.
    public static func paddedDomain(
        _ points: [MetricPoint], padding: Double, upperLimit: Double
    ) -> ClosedRange<Double> {
        let values = points.map(\.value)
        guard let low = values.min(), let high = values.max() else { return 0...upperLimit }
        var lower = max(0, low - padding)
        var upper = min(upperLimit, high + padding)
        if upper <= lower {
            lower = max(0, lower - 1)
            upper = min(upperLimit, lower + 1)
        }
        return lower...upper
    }

    /// append adds one point to a rolling window. It replaces a point that
    /// carries the same time, so a repeated reading makes no step.
    public static func append(_ point: MetricPoint, to points: [MetricPoint], limit: Int) -> [MetricPoint] {
        var next = points
        if let index = next.firstIndex(where: { $0.at == point.at }) {
            next[index] = point
        } else {
            next.append(point)
        }
        if next.count > limit {
            next.removeFirst(next.count - limit)
        }
        return next
    }
}
