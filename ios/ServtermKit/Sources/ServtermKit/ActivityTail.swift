import Foundation

/// ActivityEntry is one line that an agent reported, with the time it
/// first appeared.
public struct ActivityEntry: Sendable, Equatable, Identifiable {
    public let text: String
    public let at: Date

    public var id: String { text + "@" + String(at.timeIntervalSince1970) }
}

/// ActivityTail remembers the last lines that one agent reported. The
/// daemon serves one line for each poll and keeps no history, so the app
/// builds the history itself, the same way the terminal does. One snapshot
/// then reads as progress.
public struct ActivityTail: Sendable, Equatable {
    public static let limit = 6

    /// entries hold the newest line first.
    public private(set) var entries: [ActivityEntry] = []

    public init() {}

    public var isEmpty: Bool { entries.isEmpty }

    /// record keeps a line that differs from the newest one. A poll
    /// repeats the same line many times, and a repeat is not progress.
    public mutating func record(_ line: String?, at: Date) {
        guard let line else { return }
        let text = line.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        if entries.first?.text == text { return }
        entries.insert(ActivityEntry(text: text, at: at), at: 0)
        if entries.count > Self.limit {
            entries.removeLast(entries.count - Self.limit)
        }
    }
}

/// ReportedList tells apart the two states that a nil list and an empty
/// list carry. The daemon sends nil when it does not report a kind at all,
/// and an empty list when it reports none of them. A screen must never
/// show "none" for a fact the daemon never sent.
public enum ReportedList<Element: Equatable>: Equatable {
    case notReported
    case none
    case items([Element])

    public static func of(_ list: [Element]?) -> ReportedList<Element> {
        guard let list else { return .notReported }
        return list.isEmpty ? .none : .items(list)
    }

    /// message names the state in one sentence. The noun is singular, for
    /// example "subagent" or "task".
    public func message(for noun: String) -> String {
        switch self {
        case .notReported:
            return "The daemon does not report a \(noun) for this agent yet."
        case .none:
            return "This agent has no \(noun)."
        case .items:
            return ""
        }
    }
}
