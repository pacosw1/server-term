import Foundation
import Testing
@testable import ServtermSSH

@Suite("Terminal keys")
struct TerminalKeyTests {
    @Test("the control keys send the bytes that a shell expects")
    func controlBytes() {
        #expect(TerminalKey.escape.bytes == [0x1b])
        #expect(TerminalKey.tab.bytes == [0x09])
        #expect(TerminalKey.enter.bytes == [0x0d])
        #expect(TerminalKey.backspace.bytes == [0x7f])
    }

    @Test("the arrows send the CSI sequences")
    func arrowBytes() {
        #expect(TerminalKey.up.bytes == [0x1b, 0x5b, 0x41])
        #expect(TerminalKey.down.bytes == [0x1b, 0x5b, 0x42])
        #expect(TerminalKey.right.bytes == [0x1b, 0x5b, 0x43])
        #expect(TerminalKey.left.bytes == [0x1b, 0x5b, 0x44])
    }

    @Test("control and a letter send the control code, in either case")
    func controlLetters() {
        #expect(TerminalKey.control("c").bytes == [0x03])
        #expect(TerminalKey.control("C").bytes == [0x03])
        #expect(TerminalKey.control("d").bytes == [0x04])
        #expect(TerminalKey.control("a").bytes == [0x01])
        #expect(TerminalKey.control("z").bytes == [0x1a])
    }

    @Test("control and a bracket sends escape, as a terminal does")
    func controlBracket() {
        #expect(TerminalKey.control("[").bytes == [0x1b])
    }

    @Test("control and a key that has no code sends the key itself")
    func controlUnknown() {
        #expect(TerminalKey.control("1").bytes == Array("1".utf8))
    }

    @Test("alt and a letter send escape and then the letter")
    func altLetters() {
        #expect(TerminalKey.alt("b").bytes == [0x1b, 0x62])
        #expect(TerminalKey.alt(".").bytes == [0x1b, 0x2e])
    }

    @Test("a plain character sends its own bytes, including a pipe")
    func plainText() {
        #expect(TerminalKey.text("|").bytes == [0x7c])
        #expect(TerminalKey.text("~").bytes == [0x7e])
        #expect(TerminalKey.text("ls -la").bytes == Array("ls -la".utf8))
        #expect(TerminalKey.text("é").bytes == Array("é".utf8))
    }

    @Test("the row holds every key that iOS hides from a shell")
    func rowContents() {
        let labels = TerminalKey.row.map(\.label)
        for needed in ["esc", "tab", "ctrl", "alt", "|", "/", "\\", "-", "_", "~", ":", ";", "\"", "'"] {
            #expect(labels.contains(needed), "the row is missing \(needed)")
        }
    }

    @Test("the sticky control key clears after one press")
    func stickyControl() {
        var row = KeyRowState()
        #expect(row.isControlHeld == false)
        row.toggleControl()
        #expect(row.isControlHeld)
        #expect(row.resolve(.text("c")).bytes == [0x03])
        #expect(row.isControlHeld == false, "control must clear after it is used once")
    }

    @Test("the sticky alt key works the same way")
    func stickyAlt() {
        var row = KeyRowState()
        row.toggleAlt()
        #expect(row.resolve(.text("b")).bytes == [0x1b, 0x62])
        #expect(row.isAltHeld == false)
    }

    @Test("control and alt together send escape and then the control code")
    func controlAndAlt() {
        var row = KeyRowState()
        row.toggleControl()
        row.toggleAlt()
        #expect(row.resolve(.text("c")).bytes == [0x1b, 0x03])
        #expect(row.isControlHeld == false)
        #expect(row.isAltHeld == false)
    }

    @Test("a held control applies to a key from the system keyboard too")
    func stickyControlOnTypedText() {
        var row = KeyRowState()
        row.toggleControl()
        // The system keyboard hands over the raw byte of the letter.
        #expect(row.resolveTyped(Array("c".utf8)).bytes == [0x03])
        #expect(row.isControlHeld == false)
    }

    @Test("a held alt applies to a typed key too")
    func stickyAltOnTypedText() {
        var row = KeyRowState()
        row.toggleAlt()
        #expect(row.resolveTyped(Array("b".utf8)).bytes == [0x1b, 0x62])
    }

    @Test("typed text passes through untouched when no modifier is held")
    func plainTypedText() {
        var row = KeyRowState()
        #expect(row.resolveTyped(Array("ls".utf8)).bytes == Array("ls".utf8))
        #expect(row.resolveTyped([0x0d]).bytes == [0x0d])
    }

    @Test("a modifier does not fold a whole pasted line into one code")
    func modifierWithPaste() {
        var row = KeyRowState()
        row.toggleControl()
        let pasted = Array("echo hello".utf8)
        #expect(row.resolveTyped(pasted).bytes == pasted)
        #expect(row.isControlHeld == false, "the modifier clears, so the next key is plain")
    }
}
