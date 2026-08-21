import Combine
import SwiftUI
import ServtermKit
import ServtermSSH

/// ShellView is the terminal screen for one server. It is the only screen
/// that writes to a host, and it writes only what the user types.
struct ShellView: View {
    @Environment(AppModel.self) private var model
    @State private var shell: ShellModel = ShellFactory.makeShell()
    @State private var row = KeyRowState()
    @State private var bridge = TerminalBridge()
    @State private var screenMirror = ""
    private let mirrorTimer = Timer.publish(every: 1, on: .main, in: .common).autoconnect()
    let server: ServerEntry
    let session: String

    var body: some View {
        VStack(spacing: 0) {
            header
            if case .refused(let warning) = shell.state {
                ScrollView {
                    HostKeyWarningView(warning: warning, host: server.host)
                        .padding()
                }
            } else {
                #if UITEST_SUPPORT
                if ShellFactory.usesFakeShell {
                    // The test reads what the transport received from here.
                    Text(FakeShellLog.shared.received)
                        .accessibilityIdentifier("fake-input-log")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .lineLimit(3)
                        .padding(.horizontal, 8)
                    Text(FakeShellLog.shared.resizes)
                        .accessibilityIdentifier("fake-resize-log")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .padding(.horizontal, 8)
                    // What the terminal itself holds, read back from
                    // SwiftTerm, so a test can see that it parsed the
                    // bytes rather than trusting the model.
                    Text(screenMirror)
                        .accessibilityIdentifier("fake-screen-mirror")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .lineLimit(3)
                        .padding(.horizontal, 8)
                        .onReceive(mirrorTimer) { _ in screenMirror = bridge.screenText() }
                }
                #endif
                TerminalScreen(
                    // A key from the system keyboard goes through the same
                    // sticky control and alt state as a key from the row.
                    // Without this, holding ctrl and typing c sent the
                    // letter c instead of the interrupt.
                    onInput: { bytes in shell.send(row.resolveTyped(bytes).bytes) },
                    onResize: { columns, rows in shell.resize(columns: columns, rows: rows) },
                    bridge: bridge)
                    .background(Theme.base)
                KeyRowView(row: $row) { bytes in shell.send(bytes) }
            }
        }
        .background(Theme.base)
        .navigationTitle(session)
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            shell.prepare(comment: ShellIdentityBootstrap.comment)
            shell.onOutput = { [bridge] bytes in bridge.feed(bytes) }
            shell.connect(server: server, session: session, columns: 80, rows: 24)
        }
        .onDisappear { shell.leave() }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                StateChip(text: shell.state.label, color: stateColor, systemImage: stateIcon)
                Spacer(minLength: 8)
                Label(server.name, systemImage: "server.rack")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            Text(shell.note)
                .font(.caption)
                .foregroundStyle(Theme.muted)
                .fixedSize(horizontal: false, vertical: true)
            if let error = shell.setupError {
                ErrorBanner(message: error)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Theme.border).frame(height: Theme.borderWidth)
        }
    }

    private var stateColor: Color {
        switch shell.state {
        case .connected: return Theme.normal
        case .connecting: return Theme.warning
        case .refused, .disconnected: return Theme.critical
        case .idle, .detached: return Theme.muted
        }
    }

    private var stateIcon: String {
        switch shell.state {
        case .connected: return "checkmark.circle.fill"
        case .connecting: return "arrow.triangle.2.circlepath"
        case .refused: return "exclamationmark.triangle.fill"
        case .disconnected: return "xmark.circle.fill"
        case .idle, .detached: return "pause.circle"
        }
    }
}

/// HostKeyWarningView is the loud refusal. It offers no way to continue.
struct HostKeyWarningView: View {
    let warning: String
    let host: String

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("The host key changed", systemImage: "exclamationmark.triangle.fill")
                .font(.headline)
                .foregroundStyle(Theme.critical)
            Text(warning)
                .font(.footnote)
                .foregroundStyle(Theme.text)
                .fixedSize(horizontal: false, vertical: true)
            Text("The app will not connect. There is no way past this screen inside the app. Clear the trusted key in Settings only when you know why it changed.")
                .font(.caption)
                .foregroundStyle(Theme.muted)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(Theme.cardPadding)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .overlay {
            RoundedRectangle(cornerRadius: Theme.cardRadius)
                .strokeBorder(Theme.dangerFill, lineWidth: Theme.borderWidth)
        }
    }
}
