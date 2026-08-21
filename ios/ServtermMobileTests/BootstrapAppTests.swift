import Foundation
import Testing
import ServtermKit
@testable import ServtermMobile

/// StubFileAccess holds the import file in memory, so no test writes into
/// the real app container.
private final class StubFileAccess: FileAccess, @unchecked Sendable {
    private let lock = NSLock()
    private var files: [URL: Data]
    private(set) var removed: [URL] = []

    init(files: [URL: Data] = [:]) { self.files = files }

    func exists(at url: URL) -> Bool { lock.withLock { files[url] != nil } }

    func contents(at url: URL) throws -> Data {
        try lock.withLock {
            guard let data = files[url] else { throw CocoaError(.fileNoSuchFile) }
            return data
        }
    }

    func remove(at url: URL) throws {
        lock.withLock {
            removed.append(url)
            files[url] = nil
        }
    }
}

/// FailingTokenStore refuses every write, like a Keychain that the app
/// cannot reach. The app must report that, never treat it as a success.
private final class FailingTokenStore: TokenStore, @unchecked Sendable {
    func token(for id: String) -> String? { nil }
    func setToken(_ token: String, for id: String) -> Bool { false }
    func removeToken(for id: String) {}
}

private let folder = URL(fileURLWithPath: "/tmp/servterm-test-container")
private let importURL = folder.appendingPathComponent("servterm-import.json")

private let body = """
{"servers":[{"name":"server-a","agent_url":"http://100.64.0.1:7843","location":"Site A","token":"token-a"}],
 "orchestrator":{"name":"agents","endpoint":"http://100.64.0.1:7844","token":"token-c"}}
"""

@MainActor
@Suite("Bootstrap in the app")
struct BootstrapAppTests {
    private func makeModel(tokens: MemoryTokenStore) -> AppModel {
        AppModel(
            api: ServtermAPI(client: URLSessionHTTPClient()),
            tokens: tokens,
            defaults: UserDefaults(suiteName: "test." + UUID().uuidString)!)
    }

    @Test("the import fills the setup, saves each token, and deletes the file")
    func importsFile() {
        let store = MemoryTokenStore()
        let model = makeModel(tokens: store)
        let files = StubFileAccess(files: [importURL: Data(body.utf8)])
        model.runBootstrapImport(files: files, directory: folder)

        #expect(model.config.servers.map(\.name) == ["server-a"])
        #expect(model.config.orchestrator?.endpoint == "http://100.64.0.1:7844")
        let serverID = model.config.servers[0].id
        #expect(model.token(for: serverID) == "token-a")
        #expect(model.token(for: model.config.orchestrator!.id) == "token-c")
        #expect(files.removed == [importURL])
        #expect(model.bootstrapFailed == false)
        #expect(model.bootstrapMessage != nil)
    }

    @Test("no import file leaves the setup and the message alone")
    func noFile() {
        let model = makeModel(tokens: MemoryTokenStore())
        let files = StubFileAccess()
        model.runBootstrapImport(files: files, directory: folder)
        #expect(model.config.servers.isEmpty)
        #expect(model.bootstrapMessage == nil)
    }

    @Test("a damaged file shows an error and is still deleted")
    func damagedFile() {
        let model = makeModel(tokens: MemoryTokenStore())
        let files = StubFileAccess(files: [importURL: Data("{not json".utf8)])
        model.runBootstrapImport(files: files, directory: folder)
        #expect(model.config.servers.isEmpty)
        #expect(model.bootstrapFailed)
        #expect(model.bootstrapMessage != nil)
        #expect(files.removed == [importURL])
    }

    @Test("a second import of the same host updates the token and adds no copy")
    func secondImport() {
        let store = MemoryTokenStore()
        let model = makeModel(tokens: store)
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(body.utf8)]), directory: folder)
        let firstID = model.config.servers[0].id

        let newBody = body.replacingOccurrences(of: "token-a", with: "token-new")
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(newBody.utf8)]), directory: folder)

        #expect(model.config.servers.count == 1)
        #expect(model.config.servers[0].id == firstID)
        #expect(model.token(for: firstID) == "token-new")
    }

    @Test("a Keychain that refuses the write is reported, not treated as a save")
    func keychainFailure() {
        let model = AppModel(
            api: ServtermAPI(client: URLSessionHTTPClient()),
            tokens: FailingTokenStore(),
            defaults: UserDefaults(suiteName: "test." + UUID().uuidString)!)
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(body.utf8)]), directory: folder)
        #expect(model.bootstrapFailed)
        #expect(model.bootstrapMessage?.contains("Keychain") == true)
    }

    @Test("a failed manual save shows a message in the settings")
    func manualSaveFailure() {
        let model = AppModel(
            api: ServtermAPI(client: URLSessionHTTPClient()),
            tokens: FailingTokenStore(),
            defaults: UserDefaults(suiteName: "test." + UUID().uuidString)!)
        model.upsert(
            server: ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843"), token: "t")
        #expect(model.settingsMessage?.contains("Keychain") == true)
    }

    @Test("a good save leaves no settings message")
    func manualSaveSuccess() {
        let model = AppModel(
            api: ServtermAPI(client: URLSessionHTTPClient()),
            tokens: MemoryTokenStore(),
            defaults: UserDefaults(suiteName: "test." + UUID().uuidString)!)
        model.upsert(
            server: ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843"), token: "t")
        #expect(model.settingsMessage == nil)
    }

    @Test("a second import adds a shell account that the first one lacked")
    func importAddsShellAccount() {
        let model = makeModel(tokens: MemoryTokenStore())
        let first = """
        {"servers":[{"name":"one","agent_url":"http://100.64.0.1:7843","token":"t"}]}
        """
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(first.utf8)]), directory: folder)
        #expect(model.config.servers[0].sshUser == "")

        let second = """
        {"servers":[{"name":"one","agent_url":"http://100.64.0.1:7843","ssh_user":"root","token":"t"}]}
        """
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(second.utf8)]), directory: folder)
        #expect(model.config.servers.count == 1)
        #expect(model.config.servers[0].sshUser == "root")
    }

    @Test("an import without a shell account does not wipe one already set")
    func importKeepsShellAccount() {
        let model = makeModel(tokens: MemoryTokenStore())
        let withUser = """
        {"servers":[{"name":"one","agent_url":"http://100.64.0.1:7843","ssh_user":"root"}]}
        """
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(withUser.utf8)]), directory: folder)
        let without = """
        {"servers":[{"name":"one","agent_url":"http://100.64.0.1:7843"}]}
        """
        model.runBootstrapImport(
            files: StubFileAccess(files: [importURL: Data(without.utf8)]), directory: folder)
        #expect(model.config.servers[0].sshUser == "root")
    }
}
