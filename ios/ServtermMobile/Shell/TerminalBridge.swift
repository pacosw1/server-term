import Foundation
import SwiftTerm
import UIKit

/// TerminalBridge connects the model to the terminal view.
///
/// The screen used to hand a closure back through SwiftUI state during the
/// view update, which SwiftUI does not promise to keep, and the output
/// never reached the view: the terminal stayed blank while the session was
/// attached. A small object that the view registers itself with survives
/// every update.
@MainActor
final class TerminalBridge {
    private weak var view: TerminalView?

    func attach(_ view: TerminalView) {
        self.view = view
    }

    /// feed draws the bytes that arrived from the host.
    func feed(_ bytes: [UInt8]) {
        view?.feed(byteArray: ArraySlice(bytes))
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
