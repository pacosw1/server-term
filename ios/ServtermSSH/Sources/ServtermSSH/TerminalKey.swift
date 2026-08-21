import Foundation

/// TerminalKey is one key of the row above the keyboard, and the bytes it
/// sends. iOS hides most of these characters behind two taps, and a shell
/// needs them constantly.
public enum TerminalKey: Sendable, Equatable {
    case escape
    case tab
    case enter
    case backspace
    case up
    case down
    case left
    case right
    case control(String)
    case alt(String)
    case text(String)

    public var bytes: [UInt8] {
        switch self {
        case .escape: return [0x1b]
        case .tab: return [0x09]
        case .enter: return [0x0d]
        case .backspace: return [0x7f]
        case .up: return [0x1b, 0x5b, 0x41]
        case .down: return [0x1b, 0x5b, 0x42]
        case .right: return [0x1b, 0x5b, 0x43]
        case .left: return [0x1b, 0x5b, 0x44]
        case .control(let key): return Self.controlBytes(for: key)
        case .alt(let key): return [0x1b] + Array(key.utf8)
        case .text(let value): return Array(value.utf8)
        }
    }

    /// label is what the key row prints.
    public var label: String {
        switch self {
        case .escape: return "esc"
        case .tab: return "tab"
        case .enter: return "enter"
        case .backspace: return "del"
        case .up: return "↑"
        case .down: return "↓"
        case .left: return "←"
        case .right: return "→"
        case .control: return "ctrl"
        case .alt: return "alt"
        case .text(let value): return value
        }
    }

    /// controlBytes turns a letter into its control code. Control and C is
    /// 3, the byte that stops a program.
    private static func controlBytes(for key: String) -> [UInt8] {
        guard let scalar = key.lowercased().unicodeScalars.first else { return [] }
        switch scalar {
        case "a"..."z":
            return [UInt8(scalar.value - 0x60)]
        case "[":
            return [0x1b]
        case "\\":
            return [0x1c]
        case "]":
            return [0x1d]
        case " ", "@":
            return [0x00]
        default:
            return Array(key.utf8)
        }
    }

    /// row is the key row that the screen draws. It holds the keys that a
    /// shell needs and that iOS hides.
    public static let row: [TerminalKey] = [
        .escape, .tab, .control("c"), .alt("a"),
        .up, .down, .left, .right,
        .text("|"), .text("/"), .text("\\"), .text("-"), .text("_"),
        .text("~"), .text(":"), .text(";"), .text("\""), .text("'"),
    ]
}

/// KeyRowState holds the sticky control and alt keys. A sticky key changes
/// the next key press once, the way a terminal keyboard does.
public struct KeyRowState: Sendable, Equatable {
    public private(set) var isControlHeld = false
    public private(set) var isAltHeld = false

    public init() {}

    public mutating func toggleControl() { isControlHeld.toggle() }
    public mutating func toggleAlt() { isAltHeld.toggle() }

    /// resolveTyped applies the sticky keys to the bytes that the system
    /// keyboard produced. A terminal keyboard behaves this way: control
    /// stays held for the next key, whichever keyboard sends it.
    public mutating func resolveTyped(_ bytes: [UInt8]) -> ResolvedKey {
        guard isControlHeld || isAltHeld else { return ResolvedKey(bytes: bytes) }
        guard let text = String(bytes: bytes, encoding: .utf8), text.count == 1 else {
            // A modifier applies to one key press. Anything longer, for
            // example a paste, passes through and clears the modifiers.
            isControlHeld = false
            isAltHeld = false
            return ResolvedKey(bytes: bytes)
        }
        return resolve(.text(text))
    }

    /// resolve applies the sticky keys to one press, and then clears them.
    public mutating func resolve(_ key: TerminalKey) -> ResolvedKey {
        defer {
            isControlHeld = false
            isAltHeld = false
        }
        var bytes: [UInt8]
        if isControlHeld, case .text(let value) = key {
            bytes = TerminalKey.control(value).bytes
        } else {
            bytes = key.bytes
        }
        if isAltHeld, !bytes.isEmpty, bytes.first != 0x1b {
            bytes = [0x1b] + bytes
        } else if isAltHeld, case .text = key {
            bytes = [0x1b] + bytes
        }
        return ResolvedKey(bytes: bytes)
    }
}

/// ResolvedKey is the bytes that one press finally sends.
public struct ResolvedKey: Sendable, Equatable {
    public let bytes: [UInt8]
}
