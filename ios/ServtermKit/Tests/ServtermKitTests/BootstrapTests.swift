import Foundation
import Testing
@testable import ServtermKit

/// FakeFileAccess holds one file in memory. It records every removal, so a
/// test can prove that the app deletes the import file.
private final class FakeFileAccess: FileAccess, @unchecked Sendable {
    private let lock = NSLock()
    private var files: [URL: Data]
    private(set) var removed: [URL] = []
    var removeFails = false

    init(files: [URL: Data] = [:]) { self.files = files }

    func exists(at url: URL) -> Bool { lock.withLock { files[url] != nil } }

    func contents(at url: URL) throws -> Data {
        try lock.withLock {
            guard let data = files[url] else { throw CocoaError(.fileNoSuchFile) }
            return data
        }
    }

    func remove(at url: URL) throws {
        lock.withLock { removed.append(url) }
        if removeFails { throw CocoaError(.fileWriteNoPermission) }
        lock.withLock { files[url] = nil }
    }
}

private let importURL = URL(fileURLWithPath: "/tmp/test/servterm-import.json")

private let goodBody = """
{"servers":[
   {"name":"server-a","agent_url":"http://100.64.0.1:7843","location":"Site A","token":"token-a"},
   {"name":"server-b","agent_url":"http://100.64.0.2:7843","location":"Site B","token":"token-b"}],
 "orchestrator":{"name":"agents","endpoint":"http://100.64.0.1:7844","token":"token-c"}}
"""

@Suite("BootstrapImport")
struct BootstrapTests {
    @Test("the parser reads the servers, the orchestrator, and every token")
    func parse() throws {
        let result = try BootstrapImport.parse(Data(goodBody.utf8))
        #expect(result.config.servers.map(\.name) == ["server-a", "server-b"])
        #expect(result.config.servers[0].agentURL == "http://100.64.0.1:7843")
        #expect(result.config.servers[1].location == "Site B")
        #expect(result.config.orchestrator?.endpoint == "http://100.64.0.1:7844")
        let firstID = result.config.servers[0].id.uuidString
        let orchestratorID = try #require(result.config.orchestrator?.id.uuidString)
        #expect(result.tokens[firstID] == "token-a")
        #expect(result.tokens[result.config.servers[1].id.uuidString] == "token-b")
        #expect(result.tokens[orchestratorID] == "token-c")
        #expect(result.tokens.count == 3)
    }

    @Test("a server without an agent URL is skipped")
    func skipsBadServer() throws {
        let body = """
        {"servers":[{"name":"no-url","token":"t"},{"name":"ok","agent_url":"http://10.0.0.1:7843"}]}
        """
        let result = try BootstrapImport.parse(Data(body.utf8))
        #expect(result.config.servers.map(\.name) == ["ok"])
        #expect(result.tokens.isEmpty)
    }

    @Test("a file with no server and no orchestrator is an error")
    func emptyFile() {
        #expect(throws: ServtermError.self) {
            _ = try BootstrapImport.parse(Data("{}".utf8))
        }
    }

    @Test("damaged JSON is an error")
    func damagedJSON() {
        #expect(throws: ServtermError.self) {
            _ = try BootstrapImport.parse(Data("{not json".utf8))
        }
    }

    @Test("no file gives no result and no removal")
    func noFile() {
        let files = FakeFileAccess()
        let outcome = BootstrapImport.load(from: importURL, files: files)
        #expect(outcome == .none)
        #expect(files.removed.isEmpty)
    }

    @Test("a good file is imported and then deleted")
    func importAndDelete() throws {
        let files = FakeFileAccess(files: [importURL: Data(goodBody.utf8)])
        let outcome = BootstrapImport.load(from: importURL, files: files)
        guard case .imported(let result) = outcome else {
            Issue.record("the load did not import the file")
            return
        }
        #expect(result.config.servers.count == 2)
        #expect(files.removed == [importURL])
        #expect(files.exists(at: importURL) == false)
    }

    @Test("a damaged file is also deleted, because it holds tokens")
    func deleteAfterFailure() {
        let files = FakeFileAccess(files: [importURL: Data("{not json".utf8)])
        let outcome = BootstrapImport.load(from: importURL, files: files)
        guard case .failed = outcome else {
            Issue.record("the load did not report a failure")
            return
        }
        #expect(files.removed == [importURL])
        #expect(files.exists(at: importURL) == false)
    }

    @Test("a failed delete is reported, because the tokens stay on the disk")
    func failedDelete() {
        let files = FakeFileAccess(files: [importURL: Data(goodBody.utf8)])
        files.removeFails = true
        let outcome = BootstrapImport.load(from: importURL, files: files)
        guard case .failed(let message) = outcome else {
            Issue.record("the load did not report the failed delete")
            return
        }
        #expect(message.contains("delete"))
        #expect(files.removed == [importURL])
    }

    @Test("the real file access reads and deletes a temporary file")
    func realFileAccess() throws {
        let directory = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("servterm-test-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let url = directory.appendingPathComponent("servterm-import.json")
        try Data(goodBody.utf8).write(to: url)

        let files = SystemFileAccess()
        #expect(files.exists(at: url))
        let outcome = BootstrapImport.load(from: url, files: files)
        guard case .imported(let result) = outcome else {
            Issue.record("the load did not import the real file")
            return
        }
        #expect(result.config.servers.count == 2)
        #expect(FileManager.default.fileExists(atPath: url.path) == false)
    }
}
