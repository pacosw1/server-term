import Foundation
import Observation
import ServtermKit
import ServtermSSH

/// ShellModel runs one SSH session for one server. It is the only place in
/// the app that writes anything to a host, and it writes only the bytes
/// that the user types.
@MainActor
@Observable
final class ShellModel {
    private(set) var state: SessionState = .idle
    private(set) var note = ""
    private(set) var identity: SSHIdentity?
    private(set) var setupError: String?
    /// output carries the bytes that arrive, for the terminal view.
    var onOutput: (([UInt8]) -> Void)?

    private let client: any SSHConnecting
    private let service: TmuxSessionService
    private let identityStore: KeychainIdentityStore
    private var readTask: Task<Void, Never>?
    private var machine = SessionMachine()

    init(
        client: any SSHConnecting = NIOSSHClient(
            checker: HostKeyChecker(store: KeychainFingerprintStore())),
        service: TmuxSessionService? = nil,
        identityStore: KeychainIdentityStore = KeychainIdentityStore()
    ) {
        let runner = SSHCommandRunner(checker: HostKeyChecker(store: KeychainFingerprintStore()))
        self.client = client
        self.service = service
            ?? TmuxSessionService(
                runner: runner,
                locator: TmuxProbeLocator(runner: runner, cache: DefaultsTmuxCache()))
        self.identityStore = identityStore
    }

    /// prepare loads the key of this phone, or makes one on the first run.
    func prepare(comment: String) {
        do {
            identity = try identityStore.loadOrCreate(comment: comment)
            setupError = nil
        } catch {
            setupError = error.localizedDescription
        }
    }

    /// connect opens the session. The tmux plan makes it persistent when
    /// the host has tmux.
    func connect(server: ServerEntry, session: String, columns: Int, rows: Int) {
        guard readTask == nil else { return }
        guard !server.sshUser.isEmpty else {
            setupError = ShellError.noUser.localizedDescription
            return
        }
        guard let identity else {
            setupError = "The app has no key for this phone yet."
            return
        }
        guard TmuxSessionName.isValid(session) else {
            setupError = TmuxSessionName.rule
            return
        }
        apply(.connectStarted)
        let probeRequest = SSHRequest(
            host: server.host, user: server.sshUser, identity: identity,
            plan: SessionPlan.plainShell(reason: "probe"), columns: columns, rows: rows)
        readTask = Task { [client, service] in
            // The plan comes from the same service the session list uses,
            // so the screen and the list can never disagree about a host.
            let plan = await service.plan(session: session, request: probeRequest)
            await MainActor.run { self.note = plan.note }
            let request = SSHRequest(
                host: server.host, user: server.sshUser, identity: identity,
                plan: plan, columns: columns, rows: rows)
            for await event in client.open(request) {
                await MainActor.run {
                    switch event {
                    case .state(let state): self.state = state
                    case .output(let bytes): self.onOutput?(bytes)
                    }
                }
            }
            await MainActor.run { self.readTask = nil }
        }
    }

    /// send writes the bytes of one key press.
    func send(_ bytes: [UInt8]) {
        Task { [client] in await client.send(bytes) }
    }

    func resize(columns: Int, rows: Int) {
        Task { [client] in await client.resize(columns: columns, rows: rows) }
    }

    /// leave drops the socket. The tmux session on the host keeps running,
    /// so the work survives the app closing.
    func leave() {
        readTask?.cancel()
        readTask = nil
        Task { [client] in await client.close() }
        apply(.appLeftScreen)
    }

    private func apply(_ event: SessionEvent) {
        machine.apply(event)
        state = machine.state
    }
}
