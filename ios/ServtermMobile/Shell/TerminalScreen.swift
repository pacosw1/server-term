import SwiftTerm
import SwiftUI
import UIKit

/// TerminalScreen puts the SwiftTerm view on the screen. SwiftTerm parses
/// the escape sequences; the app writes no VT parser of its own.
///
/// Everything the view says and everything it hears goes through one
/// bridge, so the screen holds no closures that a later update could leave
/// behind.
struct TerminalScreen: UIViewRepresentable {
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
        Coordinator(bridge: bridge)
    }

    /// Coordinator hands every event of the view to the bridge. It holds
    /// the bridge itself, not a copy of a closure, so there is nothing that
    /// can point at a screen that has gone.
    @MainActor
    final class Coordinator: NSObject, TerminalViewDelegate {
        private let bridge: TerminalBridge

        init(bridge: TerminalBridge) {
            self.bridge = bridge
        }

        nonisolated func send(source: TerminalView, data: ArraySlice<UInt8>) {
            let bytes = Array(data)
            MainActor.assumeIsolated { bridge.input(bytes) }
        }

        nonisolated func sizeChanged(source: TerminalView, newCols: Int, newRows: Int) {
            MainActor.assumeIsolated { bridge.resized(columns: newCols, rows: newRows) }
        }

        nonisolated func setTerminalTitle(source: TerminalView, title: String) {}
        nonisolated func hostCurrentDirectoryUpdate(source: TerminalView, directory: String?) {}
        nonisolated func scrolled(source: TerminalView, position: Double) {}
        nonisolated func clipboardCopy(source: TerminalView, content: Data) {
            let text = String(data: content, encoding: .utf8)
            MainActor.assumeIsolated { UIPasteboard.general.string = text }
        }
        nonisolated func rangeChanged(source: TerminalView, startY: Int, endY: Int) {}
        nonisolated func requestOpenLink(source: TerminalView, link: String, params: [String: String]) {}
        nonisolated func bell(source: TerminalView) {}
        nonisolated func iTermContent(source: TerminalView, content: ArraySlice<UInt8>) {}
    }
}
