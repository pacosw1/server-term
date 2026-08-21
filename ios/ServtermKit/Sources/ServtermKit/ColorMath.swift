import Foundation

/// RGB is one colour, with each part from 0 to 1. The type holds no UI
/// framework, so the maths can be tested on its own.
public struct RGB: Sendable, Equatable {
    public let red: Double
    public let green: Double
    public let blue: Double

    public init(red: Double, green: Double, blue: Double) {
        self.red = red
        self.green = green
        self.blue = blue
    }
}

/// ColorMath holds the colour rules of the theme. The web app derives its
/// muted text, its borders and its status colours with the same steps, so
/// the phone and the web keep one look.
public enum ColorMath {
    /// parse reads a six digit hex colour, with or without the hash.
    public static func parse(_ text: String) -> RGB? {
        var value = text.trimmingCharacters(in: .whitespaces)
        if value.hasPrefix("#") { value.removeFirst() }
        guard value.count == 6, let number = UInt32(value, radix: 16) else { return nil }
        return RGB(
            red: Double((number >> 16) & 0xff) / 255,
            green: Double((number >> 8) & 0xff) / 255,
            blue: Double(number & 0xff) / 255)
    }

    /// relativeLuminance follows the WCAG 2.1 formula.
    public static func relativeLuminance(_ color: RGB) -> Double {
        func channel(_ value: Double) -> Double {
            value <= 0.04045 ? value / 12.92 : pow((value + 0.055) / 1.055, 2.4)
        }
        return 0.2126 * channel(color.red) + 0.7152 * channel(color.green)
            + 0.0722 * channel(color.blue)
    }

    /// ratio is the WCAG contrast ratio, from 1 to 21. The order of the
    /// pair does not matter.
    public static func ratio(_ first: RGB, _ second: RGB) -> Double {
        let one = relativeLuminance(first)
        let two = relativeLuminance(second)
        let lighter = max(one, two)
        let darker = min(one, two)
        return (lighter + 0.05) / (darker + 0.05)
    }

    /// mix walks from one colour to another. An amount of 0 keeps the
    /// first colour and an amount of 1 gives the second.
    public static func mix(_ from: RGB, _ to: RGB, _ amount: Double) -> RGB {
        let part = min(max(amount, 0), 1)
        return RGB(
            red: from.red + (to.red - from.red) * part,
            green: from.green + (to.green - from.green) * part,
            blue: from.blue + (to.blue - from.blue) * part)
    }

    /// ensureContrast walks a colour towards the ink until it clears the
    /// target against the background. A colour that already clears the
    /// target comes back unchanged, so a hue is never lost for nothing.
    public static func ensureContrast(
        _ color: RGB, on background: RGB, towards ink: RGB, target: Double
    ) -> RGB {
        if ratio(color, background) >= target { return color }
        var step = 0.0
        while step <= 1.0 {
            let candidate = mix(color, ink, step)
            if ratio(candidate, background) >= target { return candidate }
            step += 0.02
        }
        return ink
    }
}

/// Grade is the meaning of one capacity reading. The three steps never
/// change: normal, a warning at 70 percent, and critical at 90 percent. A
/// reading that the app does not hold is unknown, never healthy.
public enum Grade: Sendable, Equatable {
    case unknown
    case normal
    case warning
    case critical

    public static let warningLevel: Double = 70
    public static let criticalLevel: Double = 90

    public static func of(percent: Double?) -> Grade {
        guard let percent, percent.isFinite, percent >= 0 else { return .unknown }
        if percent >= criticalLevel { return .critical }
        if percent >= warningLevel { return .warning }
        return .normal
    }
}
