import SwiftUI
import ServtermSSH

/// KeyRowView is the row above the keyboard. It holds the keys that a
/// shell needs and that iOS hides, plus a sticky control and alt key and a
/// paste button.
struct KeyRowView: View {
    @Binding var row: KeyRowState
    let onKey: ([UInt8]) -> Void

    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: 6) {
                stickyButton("ctrl", isOn: row.isControlHeld) { row.toggleControl() }
                stickyButton("alt", isOn: row.isAltHeld) { row.toggleAlt() }
                key(.escape)
                key(.tab)
                key(.up)
                key(.down)
                key(.left)
                key(.right)
                ForEach(["|", "/", "\\", "-", "_", "~", ":", ";", "\"", "'"], id: \.self) { text in
                    key(.text(text))
                }
                Button("paste", action: paste)
                    .buttonStyle(KeyCapStyle(isOn: false))
                    .accessibilityLabel("paste the clipboard")
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
        }
        .scrollIndicators(.hidden)
        .background(Theme.surface)
        .overlay(alignment: .top) {
            Rectangle().fill(Theme.border).frame(height: Theme.borderWidth)
        }
    }

    private func key(_ key: TerminalKey) -> some View {
        Button(key.label) {
            onKey(row.resolve(key).bytes)
        }
        .buttonStyle(KeyCapStyle(isOn: false))
        .accessibilityLabel(name(of: key))
    }

    private func stickyButton(_ label: String, isOn: Bool, action: @escaping () -> Void) -> some View {
        Button(label, action: action)
            .buttonStyle(KeyCapStyle(isOn: isOn))
            .accessibilityLabel(label)
            .accessibilityValue(isOn ? "held for the next key" : "off")
    }

    private func paste() {
        guard let text = UIPasteboard.general.string, !text.isEmpty else { return }
        onKey(Array(text.utf8))
    }

    private func name(of key: TerminalKey) -> String {
        switch key {
        case .up: return "arrow up"
        case .down: return "arrow down"
        case .left: return "arrow left"
        case .right: return "arrow right"
        case .text(let value): return value
        default: return key.label
        }
    }
}

/// KeyCapStyle is one key of the row: square, bordered, and big enough to
/// hit with a thumb.
struct KeyCapStyle: ButtonStyle {
    let isOn: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(.subheadline, design: .monospaced))
            .foregroundStyle(isOn ? Theme.base : Theme.text)
            .frame(minWidth: 40, minHeight: Theme.minimumTapTarget)
            .padding(.horizontal, 6)
            .background(isOn ? Theme.accent : Theme.raised)
            .overlay {
                Rectangle().strokeBorder(isOn ? Theme.accent : Theme.border, lineWidth: 1.5)
            }
            .opacity(configuration.isPressed ? 0.6 : 1)
    }
}
