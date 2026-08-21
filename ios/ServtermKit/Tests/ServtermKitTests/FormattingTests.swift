import Testing
@testable import ServtermKit

@Suite("Format")
struct FormattingTests {
    @Test("bytes uses 1024 steps")
    func bytes() {
        #expect(Format.bytes(0) == "0 B")
        #expect(Format.bytes(512) == "512 B")
        #expect(Format.bytes(1024) == "1.0 KB")
        #expect(Format.bytes(1536) == "1.5 KB")
        #expect(Format.bytes(1_073_741_824) == "1.0 GB")
        #expect(Format.bytes(67_196_661_760) == "62.6 GB")
    }

    @Test("bytes rejects a negative value")
    func negativeBytes() {
        #expect(Format.bytes(-1) == "n/a")
    }

    @Test("percent keeps one decimal")
    func percent() {
        #expect(Format.percent(0) == "0.0%")
        #expect(Format.percent(50.0738) == "50.1%")
        #expect(Format.percent(100) == "100.0%")
    }

    @Test("optionalPercent shows a dash for an unknown value")
    func optionalPercent() {
        #expect(Format.optionalPercent(nil) == "—")
    }

    @Test("duration shows the two largest units")
    func duration() {
        #expect(Format.duration(seconds: 45) == "45s")
        #expect(Format.duration(seconds: 125) == "2m 5s")
        #expect(Format.duration(seconds: 7500) == "2h 5m")
        #expect(Format.duration(seconds: 348_948) == "4d 0h")
    }

    @Test("duration rejects a negative value")
    func negativeDuration() {
        #expect(Format.duration(seconds: -3) == "n/a")
    }

    @Test("rate adds a per second suffix")
    func rate() {
        #expect(Format.rate(bytesPerSecond: 0) == "0 B/s")
        #expect(Format.rate(bytesPerSecond: 2048) == "2.0 KB/s")
    }

    @Test("money marks an estimate")
    func money() {
        #expect(Format.money(7.1493, isEstimate: false) == "$7.15")
        #expect(Format.money(7.1493, isEstimate: true) == "est ~$7.15")
    }
}
