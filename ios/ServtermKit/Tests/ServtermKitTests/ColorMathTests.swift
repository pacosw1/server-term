import Foundation
import Testing
@testable import ServtermKit

@Suite("ColorMath")
struct ColorMathTests {
    @Test("a hex string parses, with or without the hash")
    func parse() throws {
        let lime = try #require(ColorMath.parse("#c2f73f"))
        #expect(abs(lime.red - 194.0 / 255) < 0.001)
        #expect(abs(lime.green - 247.0 / 255) < 0.001)
        #expect(abs(lime.blue - 63.0 / 255) < 0.001)
        #expect(ColorMath.parse("c2f73f") == lime)
        #expect(ColorMath.parse("#xyz") == nil)
        #expect(ColorMath.parse("") == nil)
    }

    @Test("the contrast ratio matches the WCAG ends")
    func ratioEnds() {
        let white = RGB(red: 1, green: 1, blue: 1)
        let black = RGB(red: 0, green: 0, blue: 0)
        #expect(abs(ColorMath.ratio(white, black) - 21) < 0.001)
        #expect(abs(ColorMath.ratio(white, white) - 1) < 0.001)
        // The order of the pair does not change the ratio.
        #expect(abs(ColorMath.ratio(black, white) - ColorMath.ratio(white, black)) < 0.001)
    }

    @Test("a known pair matches the published ratio")
    func knownRatio() throws {
        // #777777 on white is 4.48 by the WCAG formula.
        let grey = try #require(ColorMath.parse("#777777"))
        let white = try #require(ColorMath.parse("#ffffff"))
        #expect(abs(ColorMath.ratio(grey, white) - 4.48) < 0.02)
    }

    @Test("mix walks from one colour to the other")
    func mix() throws {
        let black = try #require(ColorMath.parse("#000000"))
        let white = try #require(ColorMath.parse("#ffffff"))
        #expect(ColorMath.mix(black, white, 0) == black)
        #expect(ColorMath.mix(black, white, 1) == white)
        let half = ColorMath.mix(black, white, 0.5)
        #expect(abs(half.red - 0.5) < 0.001)
    }

    @Test("ensureContrast lifts a colour until it clears the target")
    func ensureContrast() throws {
        let surface = try #require(ColorMath.parse("#141a18"))
        let text = try #require(ColorMath.parse("#ecf1ee"))
        let faded = ColorMath.mix(text, surface, 0.8)
        #expect(ColorMath.ratio(faded, surface) < 7)
        let lifted = ColorMath.ensureContrast(faded, on: surface, towards: text, target: 7)
        #expect(ColorMath.ratio(lifted, surface) >= 7)
    }

    @Test("a colour that already clears the target is left alone")
    func ensureKeepsGoodColor() throws {
        let surface = try #require(ColorMath.parse("#141a18"))
        let text = try #require(ColorMath.parse("#ecf1ee"))
        #expect(ColorMath.ensureContrast(text, on: surface, towards: text, target: 7) == text)
    }

    @Test("the grade of a capacity reading keeps its three steps")
    func grade() {
        #expect(Grade.of(percent: nil) == .unknown)
        #expect(Grade.of(percent: 0) == .normal)
        #expect(Grade.of(percent: 69.9) == .normal)
        #expect(Grade.of(percent: 70) == .warning)
        #expect(Grade.of(percent: 89.9) == .warning)
        #expect(Grade.of(percent: 90) == .critical)
        #expect(Grade.of(percent: 150) == .critical)
    }

    @Test("a reading that is not a number is unknown, never healthy")
    func gradeOfNonsense() {
        #expect(Grade.of(percent: Double.nan) == .unknown)
        #expect(Grade.of(percent: -1) == .unknown)
    }
}
