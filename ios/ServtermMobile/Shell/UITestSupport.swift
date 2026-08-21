import Foundation
import Observation
import ServtermSSH

/// UITestSupport wires the shell screens to a fake transport when the app
/// is launched with -uiTestFakeShell. It exists so a test can prove the
/// last mile that no unit test can reach: a key press in the real
/// TerminalView arriving as bytes at the transport.
///
/// It is off unless that argument is present, so a normal run never sees
/// any of it.
enum UITestSupport {
    static var usesFakeShell: Bool {
        ProcessInfo.processInfo.arguments.contains("-uiTestFakeShell")
    }

    @MainActor
    static func makeShellModel(session: String) -> ShellModel {
        ShellModel(client: FakeShellTransport.shared, service: fakeService())
    }

    @MainActor
    static func makeSessionsModel() -> SessionsModel {
        SessionsModel(service: fakeService())
    }

    private static func fakeService() -> TmuxSessionService {
        let runner = FakeShellRunner()
        return TmuxSessionService(runner: runner, locator: TmuxProbeLocator(runner: runner))
    }
}

/// FakeShellLog holds what the transport received, so a test can read it
/// from the screen.
@MainActor
@Observable
final class FakeShellLog {
    static let shared = FakeShellLog()
    private(set) var received = ""
    private(set) var resizes = ""

    func append(_ hex: String) {
        received += (received.isEmpty ? "" : " | ") + hex
    }

    func recordResize(columns: Int, rows: Int) {
        resizes += (resizes.isEmpty ? "" : " | ") + "\(columns)x\(rows)"
    }
}

/// FakeShellTransport stands in for the SSH client. It answers the same
/// protocol, records every byte it is given, and echoes it back so the
/// terminal can show it.
final class FakeShellTransport: SSHConnecting, @unchecked Sendable {
    static let shared = FakeShellTransport()

    private let lock = NSLock()
    private var continuation: AsyncStream<SSHEvent>.Continuation?

    func open(_ request: SSHRequest) -> AsyncStream<SSHEvent> {
        AsyncStream { continuation in
            self.lock.withLock { self.continuation = continuation }
            continuation.yield(.state(.connected(reattached: true)))
            continuation.yield(.output(Array("FAKE-READY\r\n".utf8)))
        }
    }

    func send(_ bytes: [UInt8]) async {
        let hex = bytes.map { String(format: "%02x", $0) }.joined(separator: " ")
        await MainActor.run { FakeShellLog.shared.append(hex) }
        // The echo proves the round trip: what the transport received comes
        // back and the terminal draws it.
        let echo = "RX " + hex + "\r\n"
        lock.withLock { continuation }?.yield(.output(Array(echo.utf8)))
    }

    func resize(columns: Int, rows: Int) async {
        await MainActor.run { FakeShellLog.shared.recordResize(columns: columns, rows: rows) }
    }

    func close() async {
        lock.withLock { continuation }?.finish()
        lock.withLock { continuation = nil }
    }
}

/// FakeShellRunner answers the commands that the session flow runs, so the
/// screens can be reached without a host.
struct FakeShellRunner: SSHRunning {
    func run(_ request: SSHRequest, command: String) async throws -> CommandResult {
        if command.contains("command -v tmux") {
            return CommandResult(stdout: "/usr/bin/tmux\n", stderr: "", exitStatus: 0)
        }
        if command.contains("list-sessions") {
            return CommandResult(
                stdout: "fake-session\t1\t0\t1787330000\t1787330100\n", stderr: "", exitStatus: 0)
        }
        return CommandResult(stdout: "", stderr: "", exitStatus: 0)
    }
}
