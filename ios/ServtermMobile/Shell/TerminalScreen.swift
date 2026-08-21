import SwiftTerm
import SwiftUI
import UIKit

/// TerminalScreen puts the SwiftTerm view on the screen. SwiftTerm parses
/// the escape sequences; the app writes no VT parser of its own.
struct TerminalScreen: UIViewRepresentable {
    let onInput: ([UInt8]) -> Void
    let onResize: (Int, Int) -> Void
    /// register hands the feed function back, so the model can push the
    /// bytes that arrive.
    let register: (@escaping ([UInt8]) -> Void) -> Void

    func makeUIView(context: Context) -> TerminalView {
        let view = TerminalView(frame: .zero)
        view.terminalDelegate = context.coordinator
        view.backgroundColor = UIColor(Theme.base)
        view.nativeBackgroundColor = UIColor(Theme.base)
        view.nativeForegroundColor = UIColor(Theme.text)
        view.font = UIFont.monospacedSystemFont(ofSize: 12, weight: .regular)
        view.isOpaque = true
        register { [weak view] bytes in
            guard let view else { return }
            view.feed(byteArray: ArraySlice(bytes))
        }
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
