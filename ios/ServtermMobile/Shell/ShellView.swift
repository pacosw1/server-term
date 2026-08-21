import SwiftUI
import ServtermKit
import ServtermSSH

/// ShellView is the terminal screen for one server. It is the only screen
/// that writes to a host, and it writes only what the user types.
struct ShellView: View {
    @Environment(AppModel.self) private var model
    @State private var shell = ShellModel()
    @State private var row = KeyRowState()
    @State private var feed: (([UInt8]) -> Void)?
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
                TerminalScreen(
                    onInput: { shell.send($0) },
                    onResize: { columns, rows in shell.resize(columns: columns, rows: rows) },
                    register: { feed = $0 })
                    .background(Theme.base)
                KeyRowView(row: $row) { bytes in shell.send(bytes) }
            }
        }
        .background(Theme.base)
        .navigationTitle(session)
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            shell.prepare(comment: ShellIdentityBootstrap.comment)
            shell.onOutput = { bytes in feed?(bytes) }
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
