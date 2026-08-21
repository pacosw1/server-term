import Foundation

/// BarMeter counts the cells of a segmented meter. The terminal draws n
/// cells and fills round(v / 100 * n) of them, so the eye counts cells
/// instead of reading a length.
public enum BarMeter {
    /// The widths that the terminal uses.
    public static let totalCPUCells = 28
    public static let coreCells = 7
    public static let wideCells = 24
    public static let compactCells = 10

    public static func filledCells(percent: Double, cells: Int) -> Int {
        guard cells > 0, percent.isFinite else { return 0 }
        let clamped = min(max(percent, 0), 100)
        return Int((clamped / 100 * Double(cells)).rounded())
    }
}

/// SparkScale says how a column height is found. A percent series uses the
/// full 0 to 100 range. A rate series has no ceiling, so it scales against
/// the busiest column in the same window.
public enum SparkScale: Sendable, Equatable {
    case percent
    case relative
}

/// SparkWindow says which readings become columns. The terminal keeps the
/// newest 18 readings and drops the rest. A wider window, for example the
/// ten minutes of a detail screen, keeps its whole span and steps evenly
/// through it instead.
public enum SparkWindow: Sendable, Equatable {
    case latest
    case spread
}

/// SparkColumn is one column of a trend. The step is 1 to 8, the same ramp
/// that the terminal draws. An offline reading keeps the lowest step and
/// carries no percent, so no screen shows a number that never existed.
public struct SparkColumn: Sendable, Equatable, Identifiable {
    public let index: Int
    public let step: Int
    public let isOffline: Bool
    public let percent: Double?

    public var id: Int { index }
}

/// SparkBars shapes a history into the discrete columns that the terminal
/// draws. The maths lives here, so a view only draws rectangles.
public enum SparkBars {
    public static let window = 18
    public static let steps = 8

    /// step follows the terminal ramp: round(v / 100 * 7) picks one of the
    /// eight characters, which is step 1 to 8 here.
    public static func step(percent: Double) -> Int {
        guard percent.isFinite else { return 1 }
        let clamped = min(max(percent, 0), 100)
        return Int((clamped / 100 * Double(steps - 1)).rounded()) + 1
    }

    /// columns turns a list of readings into columns. A list longer than
    /// the window is thinned, so both ends of the window stay on the
    /// screen.
    public static func columns(
        values: [Double], window: Int = window, scale: SparkScale = .percent,
        mode: SparkWindow = .latest
    ) -> [SparkColumn] {
        let kept = keep(values, to: window, mode: mode)
        let ceiling = scale == .relative ? (kept.map { max($0, 0) }.max() ?? 0) : 100
        return kept.enumerated().map { index, value in
            SparkColumn(
                index: index,
                step: step(percent: share(of: value, ceiling: ceiling)),
                isOffline: false,
                percent: value)
        }
    }

    /// columns from samples keeps the order in time and marks a reading
    /// that the agent could not take.
    public static func columns(
        from samples: [Sample], window: Int = window, scale: SparkScale = .percent,
        mode: SparkWindow = .latest, value: (Sample) -> Double
    ) -> [SparkColumn] {
        let ordered = samples.sorted { $0.at < $1.at }
        let kept = keep(ordered, to: window, mode: mode)
        let ceiling = scale == .relative
            ? (kept.filter(\.online).map { max(value($0), 0) }.max() ?? 0) : 100
        return kept.enumerated().map { index, sample in
            guard sample.online else {
                return SparkColumn(index: index, step: 1, isOffline: true, percent: nil)
            }
            let reading = value(sample)
            return SparkColumn(
                index: index,
                step: step(percent: share(of: reading, ceiling: ceiling)),
                isOffline: false,
                percent: reading)
        }
    }

    /// columns from chart points, for a series that is already shaped.
    public static func columns(
        points: [MetricPoint], window: Int = window, scale: SparkScale = .percent,
        mode: SparkWindow = .latest
    ) -> [SparkColumn] {
        columns(values: points.map(\.value), window: window, scale: scale, mode: mode)
    }

    private static func share(of value: Double, ceiling: Double) -> Double {
        guard ceiling > 0 else { return 0 }
        return min(max(value, 0), ceiling) / ceiling * 100
    }

    /// keep picks the readings that become columns. The latest mode drops
    /// everything before the newest window, like the terminal. The spread
    /// mode steps evenly through the whole list, so a ten minute window
    /// still shows its start and its end.
    private static func keep<T>(_ items: [T], to window: Int, mode: SparkWindow) -> [T] {
        guard window > 0 else { return [] }
        guard items.count > window else { return items }
        switch mode {
        case .latest:
            return Array(items.suffix(window))
        case .spread:
            let step = Double(items.count - 1) / Double(window - 1)
            return (0..<window).map { items[Int((Double($0) * step).rounded())] }
        }
    }
}
