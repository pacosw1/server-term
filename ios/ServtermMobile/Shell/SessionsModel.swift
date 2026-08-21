import Foundation
import Observation
import ServtermKit
import ServtermSSH

/// SessionsModel reads the tmux sessions of one server and makes or kills
/// one. Every command it runs is built by the app; no text from a host
/// ever becomes a command.
@MainActor
@Observable
final class SessionsModel {
    enum Listing: Equatable {
        case loading
        case sessions([TmuxSession])
        case failed(String)
    }

    private(set) var listing: Listing = .loading
    private(set) var busy = false
    private(set) var actionError: String?

    private let runner: any SSHRunning
    private let locator: TmuxLocator
    private let identityStore: KeychainIdentityStore

    init(
        runner: any SSHRunning = SSHCommandRunner(
            checker: HostKeyChecker(store: KeychainFingerprintStore())),
        identityStore: KeychainIdentityStore = KeychainIdentityStore()
    ) {
        self.runner = runner
        self.locator = TmuxLocator(runner: runner)
        self.identityStore = identityStore
    }

    /// refresh reads the session list. A host with no tmux server yet
    /// answers with a non-zero status and a plain message; that is the
    /// empty state, not a failure.
    func refresh(server: ServerEntry) async {
        guard let request = makeRequest(server: server) else { return }
        do {
            guard let tmux = try await locator.locate(request) else {
                listing = .failed("The app found no tmux on this host, so it keeps no session.")
                return
            }
            let result = try await runner.run(request, command: TmuxCommand.listSessions(tmux: tmux))
            switch TmuxSessionList.read(
                stdout: result.stdout, stderr: result.stderr, exitStatus: result.exitStatus)
            {
            case .sessions(let sessions):
                listing = .sessions(sessions.sorted { $0.lastActivity > $1.lastActivity })
            case .failed(let reason):
                listing = .failed(reason)
            }
        } catch {
            listing = .failed(Self.reason(error))
        }
    }

    /// create makes a session and leaves it running, so the row appears in
    /// the list at once.
    func create(name: String, server: ServerEntry) async {
        guard TmuxSessionName.isValid(name) else {
            actionError = TmuxSessionName.rule
            return
        }
        guard let request = makeRequest(server: server) else { return }
        let tmux: String?
        do {
            tmux = try await locator.locate(request)
        } catch {
            // A connection that never reached the host says so. Calling
            // that "no tmux" would name the wrong fault.
            actionError = Self.reason(error)
            return
        }
        // The detached form is the one that works here. tmux new -A -s
        // NAME needs a terminal and answers "open terminal failed: not a
        // terminal" on a reading channel, which has none. Only the shell
        // screen attaches, and that channel asks for a terminal first.
        guard let tmux, let command = TmuxCommand.create(session: name, tmux: tmux) else {
            actionError = "The app found no tmux on this host."
            return
        }
        await perform(request: request, command: command, server: server)
    }

    /// kill ends one session. It is the only command that destroys
    /// anything, and the screen asks first, naming the session.
    func kill(session: TmuxSession, server: ServerEntry) async {
        guard let request = makeRequest(server: server) else { return }
        let tmux: String?
        do {
            tmux = try await locator.locate(request)
        } catch {
            actionError = Self.reason(error)
            return
        }
        guard let tmux, let command = TmuxCommand.kill(session: session.name, tmux: tmux) else {
            actionError = "The app found no tmux on this host."
            return
        }
        await perform(request: request, command: command, server: server)
    }

    private func perform(request: SSHRequest, command: String, server: ServerEntry) async {
        busy = true
        actionError = nil
        do {
            let result = try await runner.run(request, command: command)
            if result.exitStatus != 0 {
                let message = result.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
                actionError = message.isEmpty
                    ? "the command ended with status \(result.exitStatus)" : message
            }
        } catch {
            actionError = Self.reason(error)
        }
        busy = false
        await refresh(server: server)
    }

    private func makeRequest(server: ServerEntry) -> SSHRequest? {
        guard !server.sshUser.isEmpty else {
            listing = .failed("This server has no shell account yet. Set one in Settings.")
            return nil
        }
        guard let identity = try? identityStore.loadOrCreate(comment: ShellIdentityBootstrap.comment)
        else {
            listing = .failed("The app has no key for this phone yet.")
            return nil
        }
        return SSHRequest(
            host: server.host, user: server.sshUser, identity: identity,
            plan: SessionPlan.plainShell(reason: "probe"), columns: 80, rows: 24)
    }

    private static func reason(_ error: any Error) -> String {
        if let error = error as? SSHClientError { return error.message }
        return error.localizedDescription
    }
}
