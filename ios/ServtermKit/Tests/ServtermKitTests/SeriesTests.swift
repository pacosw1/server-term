import Foundation
import Testing
@testable import ServtermKit

@Suite("MetricSeries")
struct SeriesTests {
    private func sample(at seconds: TimeInterval, cpu: Double, memTotal: UInt64 = 1000,
                        memAvailable: UInt64 = 400, rx: Double = 10, tx: Double = 20) -> Sample {
        var value = Sample.empty
        value.at = Date(timeIntervalSince1970: seconds)
        value.online = true
        value.cpuPercent = cpu
        value.memTotal = memTotal
        value.memAvailable = memAvailable
        value.netRxRate = rx
        value.netTxRate = tx
        return value
    }

    @Test("the CPU series keeps the order in time")
    func cpuSeries() {
        let samples = [sample(at: 30, cpu: 5), sample(at: 10, cpu: 1), sample(at: 20, cpu: 3)]
        let series = MetricSeries.cpu(from: samples)
        #expect(series.map(\.value) == [1, 3, 5])
        #expect(series.first?.at == Date(timeIntervalSince1970: 10))
    }

    @Test("an empty history gives an empty series")
    func emptySeries() {
        #expect(MetricSeries.cpu(from: []).isEmpty)
        #expect(MetricSeries.memory(from: []).isEmpty)
    }

    @Test("a sample with no memory size is left out, not drawn as zero")
    func skipsUnknownMemory() {
        let samples = [sample(at: 10, cpu: 1), sample(at: 20, cpu: 2, memTotal: 0, memAvailable: 0)]
        let series = MetricSeries.memory(from: samples)
        #expect(series.count == 1)
        #expect(abs((series.first?.value ?? 0) - 60) < 0.001)
    }

    @Test("an offline sample is left out of every series")
    func skipsOffline() {
        var offline = sample(at: 20, cpu: 90)
        offline.online = false
        #expect(MetricSeries.cpu(from: [sample(at: 10, cpu: 1), offline]).count == 1)
    }

    @Test("the network series carries the receive rate and the send rate")
    func networkSeries() {
        let series = MetricSeries.network(from: [sample(at: 10, cpu: 1, rx: 100, tx: 200)])
        #expect(series.count == 2)
        #expect(series.filter { $0.name == "receive" }.first?.value == 100)
        #expect(series.filter { $0.name == "send" }.first?.value == 200)
    }

    @Test("a rolling window keeps only the newest points")
    func rollingWindow() {
        var points: [MetricPoint] = []
        for index in 0..<10 {
            points = MetricSeries.append(
                MetricPoint(at: Date(timeIntervalSince1970: TimeInterval(index)), value: Double(index)),
                to: points, limit: 4)
        }
        #expect(points.count == 4)
        #expect(points.map(\.value) == [6, 7, 8, 9])
    }

    @Test("the window replaces a point that has the same time")
    func noDuplicateTime() {
        let first = MetricPoint(at: Date(timeIntervalSince1970: 5), value: 1)
        let again = MetricPoint(at: Date(timeIntervalSince1970: 5), value: 2)
        let points = MetricSeries.append(again, to: [first], limit: 10)
        #expect(points.map(\.value) == [2])
    }




}
