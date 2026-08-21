import Foundation

/// Format turns raw readings into short text for the screen. Every function
/// shows a clear mark for an unknown or an impossible value. It never shows
/// a zero in place of a value that the app does not know.
public enum Format {
    public static let unknown = "—"

    /// bytes shows a byte count in 1024 steps.
    public static func bytes(_ value: Int64) -> String {
        if value < 0 { return "n/a" }
        if value < 1024 { return "\(value) B" }
        var amount = Double(value)
        let units = ["KB", "MB", "GB", "TB", "PB"]
        var index = -1
        while amount >= 1024, index < units.count - 1 {
            amount /= 1024
            index += 1
        }
        return String(format: "%.1f %@", amount, units[index])
    }

    public static func bytes(unsigned value: UInt64) -> String {
        value > UInt64(Int64.max) ? "n/a" : bytes(Int64(value))
    }

    /// optionalBytes shows the unknown mark when the app has no reading.
    public static func optionalBytes(_ value: Int64?) -> String {
        guard let value else { return unknown }
        return bytes(value)
    }

    /// rate shows a transfer speed.
    public static func rate(bytesPerSecond: Double) -> String {
        if bytesPerSecond < 0 || !bytesPerSecond.isFinite { return "n/a" }
        return bytes(Int64(bytesPerSecond)) + "/s"
    }

    /// rate with a known flag shows the dash while the app still has only
    /// one reading. A rate needs two readings, so a 0 on the first frame
    /// means "not known yet", not "idle".
    public static func rate(bytesPerSecond: Double, known: Bool) -> String {
        known ? rate(bytesPerSecond: bytesPerSecond) : unknown
    }

    /// celsius shows one temperature. The agent sends no limit with it, so
    /// the app never grades it.
    public static func celsius(_ value: Double) -> String {
        value.isFinite ? String(format: "%.1f °C", value) : "n/a"
    }

    public static func percent(_ value: Double) -> String {
        if !value.isFinite { return "n/a" }
        return String(format: "%.1f%%", value)
    }

    /// optionalPercent shows the unknown mark when the app has no reading.
    public static func optionalPercent(_ value: Double?) -> String {
        guard let value else { return unknown }
        return percent(value)
    }

    /// duration shows the two largest units of a time span.
    public static func duration(seconds: Double) -> String {
        if seconds < 0 || !seconds.isFinite { return "n/a" }
        let total = Int(seconds)
        let days = total / 86400
        let hours = (total % 86400) / 3600
        let minutes = (total % 3600) / 60
        let rest = total % 60
        if days > 0 { return "\(days)d \(hours)h" }
        if hours > 0 { return "\(hours)h \(minutes)m" }
        if minutes > 0 { return "\(minutes)m \(rest)s" }
        return "\(rest)s"
    }

    /// money shows a dollar figure. A figure that the daemon computes from
    /// token use is an estimate, not a charge, so it carries the "est ~"
    /// mark. Real billed spend stays a plain figure.
    public static func money(_ value: Double, isEstimate: Bool) -> String {
        let plain = String(format: "$%.2f", value)
        return isEstimate ? "est ~" + plain : plain
    }

    /// cores turns a CPU percent into whole cores. 100 percent is one core.
    public static func cores(cpuPercent: Double) -> String {
        if !cpuPercent.isFinite || cpuPercent < 0 { return "n/a" }
        return String(format: "%.2f cores", cpuPercent / 100)
    }

    /// relativeAge says how old a reading is.
    public static func relativeAge(_ date: Date, now: Date = Date()) -> String {
        let seconds = now.timeIntervalSince(date)
        if seconds < 0 { return "just now" }
        if seconds < 2 { return "1s ago" }
        return duration(seconds: seconds) + " ago"
    }
}
