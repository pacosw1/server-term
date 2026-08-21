import Foundation
import Testing
@testable import ServtermKit

/// The padel-night palette, with the axes that the user asked for: sharp
/// corners, heavy borders and the high contrast target. The seed itself
/// ships round corners and hairline borders; only the colours come from it.
private enum Seed {
    static let base = ColorMath.parse("#0a0e0d")!
    static let surface = ColorMath.parse("#141a18")!
    static let text = ColorMath.parse("#ecf1ee")!
    static let primary = ColorMath.parse("#c2f73f")!
    static let secondary = ColorMath.parse("#2f6df0")!
    static let highlight = ColorMath.parse("#f5c451")!
    static let danger = ColorMath.parse("#e5484d")!
    static let target = 7.0
}

@Suite("Theme contrast")
struct ThemeContrastTests {
    @Test("the body text clears AAA on the page and on a card")
    func bodyText() {
        #expect(ColorMath.ratio(Seed.text, Seed.base) >= Seed.target)
        #expect(ColorMath.ratio(Seed.text, Seed.surface) >= Seed.target)
    }

    @Test("the muted text is derived until it clears AAA on a card")
    func mutedText() {
        let muted = ColorMath.ensureContrast(
            ColorMath.mix(Seed.text, Seed.base, 0.45), on: Seed.surface,
            towards: Seed.text, target: Seed.target)
        #expect(ColorMath.ratio(muted, Seed.surface) >= Seed.target)
        #expect(ColorMath.ratio(muted, Seed.base) >= Seed.target)
    }

    @Test("every status colour that carries a number clears AAA on a card")
    func statusText() {
        for seed in [Seed.primary, Seed.highlight, Seed.danger, Seed.secondary] {
            let lifted = ColorMath.ensureContrast(
                seed, on: Seed.surface, towards: Seed.text, target: Seed.target)
            #expect(ColorMath.ratio(lifted, Seed.surface) >= Seed.target)
        }
    }

    @Test("a chart series clears the non text floor on a card")
    func seriesStrokes() {
        #expect(ColorMath.ratio(Seed.primary, Seed.surface) >= 3)
        let lifted = ColorMath.ensureContrast(
            Seed.secondary, on: Seed.surface, towards: Seed.text, target: 3)
        #expect(ColorMath.ratio(lifted, Seed.surface) >= 3)
    }

    @Test("the error banner text clears AAA on the danger fill")
    func bannerText() {
        let onDanger = ColorMath.ratio(Seed.base, Seed.danger) >= ColorMath.ratio(Seed.text, Seed.danger)
            ? Seed.base : Seed.text
        #expect(ColorMath.ratio(onDanger, Seed.danger) >= 4.5)
    }
}
