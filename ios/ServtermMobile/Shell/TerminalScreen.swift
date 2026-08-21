import SwiftTerm
import SwiftUI
import UIKit

/// TerminalScreen puts the SwiftTerm view on the screen. SwiftTerm parses
/// the escape sequences; the app writes no VT parser of its own.
struct TerminalScreen: UIViewRepresentable {
    let onInput: ([UInt8]) -> Void
    let onResize: (Int, Int) -> Void
    /// bridge is the object that carries the bytes of the host into this
    /// view. The view registers itself with it, so nothing depends on a
    /// closure handed back during a view update.
    let bridge: TerminalBridge

    func makeUIView(context: Context) -> TerminalView {
        let view = TerminalView(frame: .zero)
        view.terminalDelegate = context.coordinator
        view.backgroundColor = UIColor(Theme.base)
        view.nativeBackgroundColor = UIColor(Theme.base)
        view.nativeForegroundColor = UIColor(Theme.text)
        view.font = UIFont.monospacedSystemFont(ofSize: 12, weight: .regular)
        view.isOpaque = true
        view.accessibilityIdentifier = "terminal-view"
        bridge.attach(view)
        // The keyboard must come up by itself. A person who opens a shell
        // wants to type, and hunting for the tap that raises the keyboard
        // reads as a dead terminal.
        DispatchQueue.main.async { _ = view.becomeFirstResponder() }
        return view
    }

    func updateUIView(_ view: TerminalView, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(onInput: onInput, onResize: onResize)
    }

    /// Coordinator carries the keystrokes and the size changes back.
    final class Coordinator: NSObject, TerminalViewDelegate {
        private let onInput: ([UInt8]) -> Void
        private let onResize: (Int, Int) -> Void

        init(onInput: @escaping ([UInt8]) -> Void, onResize: @escaping (Int, Int) -> Void) {
            self.onInput = onInput
            self.onResize = onResize
        }

        func send(source: TerminalView, data: ArraySlice<UInt8>) {
            onInput(Array(data))
        }

        func sizeChanged(source: TerminalView, newCols: Int, newRows: Int) {
            onResize(newCols, newRows)
        }

        func setTerminalTitle(source: TerminalView, title: String) {}
        func hostCurrentDirectoryUpdate(source: TerminalView, directory: String?) {}
        func scrolled(source: TerminalView, position: Double) {}
        func clipboardCopy(source: TerminalView, content: Data) {
            UIPasteboard.general.string = String(data: content, encoding: .utf8)
        }
        func rangeChanged(source: TerminalView, startY: Int, endY: Int) {}
        func requestOpenLink(source: TerminalView, link: String, params: [String: String]) {}
        func bell(source: TerminalView) {}
        func iTermContent(source: TerminalView, content: ArraySlice<UInt8>) {}
    }
}
