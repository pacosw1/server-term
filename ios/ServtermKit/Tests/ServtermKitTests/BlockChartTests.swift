import Foundation
import Testing
@testable import ServtermKit

@Suite("Block charts")
struct BlockChartTests {
    // MARK: - the segmented meter

    @Test("the filled cells follow the terminal rule")
    func filledCells() {
        #expect(BarMeter.filledCells(percent: 0, cells: 28) == 0)
        #expect(BarMeter.filledCells(percent: 100, cells: 28) == 28)
        #expect(BarMeter.filledCells(percent: 50, cells: 28) == 14)
        // 12.3 percent of a 7 cell bar rounds to one cell, as CPU00 does.
        #expect(BarMeter.filledCells(percent: 12.3, cells: 7) == 1)
        #expect(BarMeter.filledCells(percent: 100, cells: 7) == 7)
    }

    @Test("the meter clamps outside 0 and 100")
    func clampedCells() {
        #expect(BarMeter.filledCells(percent: -5, cells: 28) == 0)
        #expect(BarMeter.filledCells(percent: 150, cells: 28) == 28)
        #expect(BarMeter.filledCells(percent: Double.nan, cells: 28) == 0)
        #expect(BarMeter.filledCells(percent: 50, cells: 0) == 0)
    }

    // MARK: - the trend columns

    @Test("one column takes one of eight steps")
    func columnSteps() {
        #expect(SparkBars.step(percent: 0) == 1)
        #expect(SparkBars.step(percent: 100) == 8)
        #expect(SparkBars.step(percent: 50) == 5)
        #expect(SparkBars.step(percent: 12.5) == 2)
        #expect(SparkBars.step(percent: -10) == 1)
        #expect(SparkBars.step(percent: 200) == 8)
    }

    @Test("the window keeps the last 18 readings")
    func window() {
        let values = (0..<40).map { Double($0) }
        let columns = SparkBars.columns(values: values, window: 18)
        #expect(columns.count == 18)
        #expect(columns.last?.percent == 39)
        #expect(columns.first?.percent == 22)
    }

    @Test("a short history keeps every reading it holds")
    func shortWindow() {
        #expect(SparkBars.columns(values: [1, 2, 3], window: 18).count == 3)
        #expect(SparkBars.columns(values: [], window: 18).isEmpty)
    }

    @Test("an offline reading draws a low mark, not a gap and not a zero")
    func offlineColumn() {
        var offline = Sample.empty
        offline.at = Date(timeIntervalSince1970: 20)
        offline.online = false
        offline.cpuPercent = 99
        var online = Sample.empty
        online.at = Date(timeIntervalSince1970: 10)
        online.online = true
        online.cpuPercent = 40
        let columns = SparkBars.columns(from: [online, offline], window: 18) { $0.cpuPercent }
        #expect(columns.count == 2)
        #expect(columns[0].isOffline == false)
        #expect(columns[1].isOffline)
        #expect(columns[1].step == 1)
        // An offline reading carries no percent, so no screen shows one.
        #expect(columns[1].percent == nil)
    }

    @Test("the columns follow the order in time")
    func columnOrder() {
        var first = Sample.empty
        first.at = Date(timeIntervalSince1970: 10)
        first.online = true
        first.cpuPercent = 10
        var second = Sample.empty
        second.at = Date(timeIntervalSince1970: 20)
        second.online = true
        second.cpuPercent = 90
        let columns = SparkBars.columns(from: [second, first], window: 18) { $0.cpuPercent }
        #expect(columns.map(\.percent) == [10, 90])
    }

    @Test("a rate series scales against the busiest column in the window")
    func relativeScale() {
        let columns = SparkBars.columns(values: [0, 250, 500], window: 18, scale: .relative)
        #expect(columns.map(\.step) == [1, 5, 8])
        // The percent stays the real value, so a label never shows the scale.
        #expect(columns.last?.percent == 500)
    }

    @Test("a flat rate series does not divide by zero")
    func flatRelativeScale() {
        let columns = SparkBars.columns(values: [0, 0, 0], window: 18, scale: .relative)
        #expect(columns.map(\.step) == [1, 1, 1])
    }

    @Test("a wide window is thinned to the column count, keeping both ends")
    func downsampleToColumns() {
        let values = (0..<600).map { Double($0) / 6 }
        let columns = SparkBars.columns(values: values, window: 36, mode: .spread)
        #expect(columns.count == 36)
        #expect(columns.first?.percent == 0)
        #expect(abs((columns.last?.percent ?? 0) - 599.0 / 6) < 0.001)
    }

    @Test("the latest window drops everything before it, like the terminal")
    func latestWindow() {
        let columns = SparkBars.columns(values: (0..<40).map(Double.init), window: 18, mode: .latest)
        #expect(columns.map(\.percent) == (22...39).map(Double.init))
    }

    // MARK: - the grade change

    @Test("the grade warns at 75, to match the bars in the terminal")
    func gradeThresholds() {
        #expect(Grade.of(percent: 74.9) == .normal)
        #expect(Grade.of(percent: 75) == .warning)
        #expect(Grade.of(percent: 89.9) == .warning)
        #expect(Grade.of(percent: 90) == .critical)
        #expect(Grade.warningLevel == 75)
    }
}
