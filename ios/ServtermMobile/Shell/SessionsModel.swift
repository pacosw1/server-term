import Foundation
import Observation
import ServtermKit
import ServtermSSH

/// SessionsModel is the screen state around TmuxSessionService. The flow
/// itself lives in ServtermSSH, so a harness can drive the same functions
/// against a real host before anybody tries them on a phone.
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

    private let service: TmuxSessionService
    private let identityStore: KeychainIdentityStore

    init(
        service: TmuxSessionService? = nil,
        identityStore: KeychainIdentityStore = KeychainIdentityStore()
    ) {
        let runner = SSHCommandRunner(checker: HostKeyChecker(store: KeychainFingerprintStore()))
        self.service = service
            ?? TmuxSessionService(
                runner: runner,
                locator: TmuxProbeLocator(runner: runner, cache: DefaultsTmuxCache()))
        self.identityStore = identityStore
    }

    func refresh(server: ServerEntry) async {
        guard let request = makeRequest(server: server) else { return }
        switch await service.list(request) {
        case .sessions(let sessions): listing = .sessions(sessions)
        case .failed(let reason): listing = .failed(reason)
        }
    }

    func create(name: String, server: ServerEntry) async {
        guard let request = makeRequest(server: server) else { return }
        busy = true
        actionError = await service.create(name: name, request: request)
        busy = false
        await refresh(server: server)
    }

    /// kill ends one session. The screen asks first, naming the session.
    func kill(session: TmuxSession, server: ServerEntry) async {
        guard let request = makeRequest(server: server) else { return }
        busy = true
        actionError = await service.kill(name: session.name, request: request)
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
}
