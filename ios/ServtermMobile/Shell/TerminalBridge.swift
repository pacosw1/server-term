import Foundation
import SwiftTerm
import UIKit

/// TerminalBridge is the ONE connector between the model and the terminal
/// view, in both directions.
///
/// Inbound, the model feeds bytes that the host sent. Outbound, the view
/// reports the keys a person pressed and the size the terminal took. The
/// screen sets the two outbound handlers once, when it appears, and the
/// view registers itself here in makeUIView.
///
/// The screen used to hand a closure back through SwiftUI state during the
/// view update, which SwiftUI does not promise to keep, and the terminal
/// stayed blank. The view then also kept a second pair of closures captured
/// in its coordinator. One object for both directions removes both the
/// blank screen and the risk that a captured closure quietly goes stale.
@MainActor
final class TerminalBridge {
    private weak var view: TerminalView?

    /// onInput carries the bytes of one key press to the model.
    var onInput: (([UInt8]) -> Void)?
    /// onResize carries the new terminal size to the model.
    var onResize: ((Int, Int) -> Void)?

    func attach(_ view: TerminalView) {
        self.view = view
    }

    /// feed draws the bytes that arrived from the host.
    func feed(_ bytes: [UInt8]) {
        view?.feed(byteArray: ArraySlice(bytes))
    }

    /// input is what the terminal view calls when a person types.
    func input(_ bytes: [UInt8]) {
        onInput?(bytes)
    }

    /// resized is what the terminal view calls when its size changes.
    func resized(columns: Int, rows: Int) {
        onResize?(columns, rows)
    }

    /// screenText reads what the terminal itself holds. A test uses it to
    /// prove that SwiftTerm received and parsed the bytes, not only that
    /// the model carried them.
    func screenText() -> String {
        guard let terminal = view?.getTerminal() else { return "" }
        var lines: [String] = []
        for row in 0..<terminal.rows {
            guard let line = terminal.getLine(row: row) else { continue }
            let text = line.translateToString(trimRight: true)
            if !text.isEmpty { lines.append(text) }
        }
        return lines.joined(separator: "\n")
    }
}
