import SwiftUI

/// Theme holds the one palette and the one set of shapes that every screen
/// uses. A reading keeps the same colour on every screen: green is normal,
/// orange is a warning at 70 percent, red is critical at 90 percent, and
/// the accent blue is a neutral series.
enum Theme {
    static let normal = Color(red: 0.16, green: 0.72, blue: 0.45)
    static let warning = Color(red: 0.95, green: 0.62, blue: 0.16)
    static let critical = Color(red: 0.92, green: 0.30, blue: 0.31)
    static let accent = Color(red: 0.29, green: 0.56, blue: 0.98)
    static let violet = Color(red: 0.55, green: 0.44, blue: 0.94)

    static let warningLevel: Double = 70
    static let criticalLevel: Double = 90

    static let cardRadius: CGFloat = 18
    static let cardPadding: CGFloat = 16
    static let cardSpacing: CGFloat = 12
    static let meterHeight: CGFloat = 10
    static let sparklineHeight: CGFloat = 34
    static let chartHeight: CGFloat = 150

    /// color grades one percent reading. An unknown reading keeps the
    /// neutral accent, because the app must not paint it as healthy.
    static func color(for percent: Double?) -> Color {
        guard let percent else { return .secondary }
        if percent >= criticalLevel { return critical }
        if percent >= warningLevel { return warning }
        return normal
    }

    static func gradient(for percent: Double?) -> LinearGradient {
        let base = color(for: percent)
        return LinearGradient(
            colors: [base.opacity(0.75), base], startPoint: .leading, endPoint: .trailing)
    }

    /// seriesGradient fills the area under a chart line.
    static func seriesGradient(_ base: Color) -> LinearGradient {
        LinearGradient(
            colors: [base.opacity(0.35), base.opacity(0.02)],
            startPoint: .top, endPoint: .bottom)
    }
}
