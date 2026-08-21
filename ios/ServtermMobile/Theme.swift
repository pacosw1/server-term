import SwiftUI
import ServtermKit

extension Color {
    /// A colour from the theme maths. Every colour in the app comes from
    /// here, so no view holds a literal colour.
    init(_ rgb: RGB) {
        self.init(red: rgb.red, green: rgb.green, blue: rgb.blue)
    }
}

/// Theme is the padel-night palette with the shape that the user asked
/// for: sharp corners, heavy borders and the AAA contrast target. The seed
/// colours come from the web app; the derived colours use the same maths,
/// so the phone and the web keep one look.
///
/// The theme is dark by design. The app forces the dark appearance, so no
/// view needs a second palette.
enum Theme {
    // MARK: - the seed

    static let baseRGB = ColorMath.parse("#0a0e0d")!
    static let surfaceRGB = ColorMath.parse("#141a18")!
    static let textRGB = ColorMath.parse("#ecf1ee")!
    static let primaryRGB = ColorMath.parse("#c2f73f")!
    static let secondaryRGB = ColorMath.parse("#2f6df0")!
    static let highlightRGB = ColorMath.parse("#f5c451")!
    static let dangerRGB = ColorMath.parse("#e5484d")!

    /// The high contrast axis asks for the AAA body target.
    static let bodyTarget: Double = 7
    /// A stroke or a chart line is not text, so it holds the lower floor.
    static let nonTextTarget: Double = 3

    // MARK: - the derived colours

    /// muted is the faded body text. It is lifted until it clears AAA on a
    /// card, the tighter of the two backgrounds.
    private static let mutedRGB = ColorMath.ensureContrast(
        ColorMath.mix(textRGB, baseRGB, 0.45), on: surfaceRGB, towards: textRGB, target: bodyTarget)
    /// raised is a lifted neutral for a chip or an inner row.
    private static let raisedRGB = ColorMath.mix(surfaceRGB, textRGB, 0.08)

    static let base = Color(baseRGB)
    static let surface = Color(surfaceRGB)
    static let text = Color(textRGB)
    static let muted = Color(mutedRGB)
    static let raised = Color(raisedRGB)

    /// The border is the text colour at the heavy alpha, lifted by the high
    /// contrast axis: 0.32 by 1.7, held under 0.6.
    static let border = Color(textRGB).opacity(min(0.6, 0.32 * 1.7))
    /// A control reads one step stronger than a divider.
    static let controlBorder = Color(textRGB).opacity(min(0.72, (0.32 + 0.04) * 1.7))

    // MARK: - the meaning colours

    /// normal, warning and critical keep the meaning they always had. Only
    /// the colour changed: normal is the lime primary now.
    static let normal = Color(primaryRGB)
    static let warning = Color(highlightRGB)
    static let critical = Color(
        ColorMath.ensureContrast(dangerRGB, on: surfaceRGB, towards: textRGB, target: bodyTarget))
    /// dangerFill is the raw danger colour, for a border or a fill, where
    /// no text sits on it.
    static let dangerFill = Color(dangerRGB)

    /// accent is the neutral series colour and the tint of every control.
    static let accent = Color(primaryRGB)
    /// series2 is the second chart series, lifted until it is readable on a
    /// card.
    static let series2 = Color(
        ColorMath.ensureContrast(secondaryRGB, on: surfaceRGB, towards: textRGB, target: bodyTarget))

    // MARK: - the measurements

    /// The sharp shape is 0.125rem, which is 2 points.
    static let cardRadius: CGFloat = 2
    /// The heavy border is 2 pixels.
    static let borderWidth: CGFloat = 2
    static let cardPadding: CGFloat = 16
    static let cardSpacing: CGFloat = 12
    static let meterHeight: CGFloat = 12
    static let sparklineHeight: CGFloat = 36
    static let chartHeight: CGFloat = 150
    /// The smallest tap target that Apple allows.
    static let minimumTapTarget: CGFloat = 44

    // MARK: - grading

    static func color(for grade: Grade) -> Color {
        switch grade {
        case .unknown: return muted
        case .normal: return normal
        case .warning: return warning
        case .critical: return critical
        }
    }

    /// color grades one capacity reading. A reading that can pass 100, for
    /// example a process CPU, must not go through here: it uses accent.
    static func color(for percent: Double?) -> Color {
        color(for: Grade.of(percent: percent))
    }

    /// fillColor is for a bar, a dot or any block that carries no text on
    /// it. It uses the raw seed colour, which is stronger than the lifted
    /// text colour and still clears the non text floor.
    static func fillColor(for percent: Double?) -> Color {
        switch Grade.of(percent: percent) {
        case .unknown: return muted
        case .normal: return Color(primaryRGB)
        case .warning: return Color(highlightRGB)
        case .critical: return Color(dangerRGB)
        }
    }

    /// meterFill keeps the flat look: one solid colour, no soft gradient.
    static func meterFill(_ percent: Double?) -> Color {
        fillColor(for: percent)
    }

    /// seriesGradient fills the area under a chart line. It stays faint, so
    /// the hard border of the card keeps the edge.
    static func seriesGradient(_ base: Color) -> LinearGradient {
        LinearGradient(
            colors: [base.opacity(0.30), base.opacity(0.03)],
            startPoint: .top, endPoint: .bottom)
    }
}
