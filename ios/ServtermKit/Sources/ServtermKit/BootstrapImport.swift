import Foundation

/// FileAccess hides the file system behind three functions, so a test uses
/// a double or a temporary file instead of the real app container.
public protocol FileAccess: Sendable {
    func exists(at url: URL) -> Bool
    func contents(at url: URL) throws -> Data
    func remove(at url: URL) throws
}

/// SystemFileAccess is the real file system.
public struct SystemFileAccess: FileAccess {
    public init() {}

    public func exists(at url: URL) -> Bool {
        FileManager.default.fileExists(atPath: url.path)
    }

    public func contents(at url: URL) throws -> Data {
        try Data(contentsOf: url)
    }

    public func remove(at url: URL) throws {
        try FileManager.default.removeItem(at: url)
    }
}

/// BootstrapOutcome is the result of one look at the import file.
public enum BootstrapOutcome: Sendable, Equatable {
    /// none means that the app found no import file.
    case none
    case imported(BootstrapImport.Result)
    /// failed carries a message for the screen. It never carries a token.
    case failed(String)
}

/// BootstrapImport reads one JSON file that carries the whole setup, with
/// the tokens. The file is a one-time hand over from the desktop machine.
/// The app deletes the file at once, because the file holds plain tokens.
public enum BootstrapImport {
    /// Result holds the setup and the tokens. The token map uses the id of
    /// each new entry, which is the same key that the Keychain uses.
    public struct Result: Sendable, Equatable {
        public let config: AppConfig
        public let tokens: [String: String]
    }

    /// fileName is the name that the app looks for in its Documents folder.
    public static let fileName = "servterm-import.json"

    private struct File: Decodable {
        struct Server: Decodable {
            let name: String?
            let agentURL: String?
            let location: String?
            let token: String?
            let sshUser: String?

            enum CodingKeys: String, CodingKey {
                case name
                case agentURL = "agent_url"
                case location
                case token
                case sshUser = "ssh_user"
            }
        }

        struct Orchestrator: Decodable {
            let name: String?
            let endpoint: String?
            let token: String?
        }

        let servers: [Server]?
        let orchestrator: Orchestrator?
    }

    public static func parse(_ data: Data) throws -> Result {
        let file: File
        do {
            file = try JSONDecoder().decode(File.self, from: data)
        } catch {
            // The message names the fault only. It never repeats the file
            // content, because the content holds tokens.
            throw ServtermError.importFailed("the file is not valid JSON")
        }
        var config = AppConfig()
        var tokens: [String: String] = [:]
        for server in file.servers ?? [] {
            guard let url = server.agentURL, !url.isEmpty else { continue }
            let entry = ServerEntry(
                name: server.name ?? url, agentURL: url, location: server.location ?? "",
                sshUser: server.sshUser ?? "")
            config.servers.append(entry)
            if let token = server.token, !token.isEmpty {
                tokens[entry.id.uuidString] = token
            }
        }
        if let orchestrator = file.orchestrator, let endpoint = orchestrator.endpoint,
            !endpoint.isEmpty
        {
            let entry = OrchestratorEntry(name: orchestrator.name ?? "orchestrator", endpoint: endpoint)
            config.orchestrator = entry
            if let token = orchestrator.token, !token.isEmpty {
                tokens[entry.id.uuidString] = token
            }
        }
        guard !config.servers.isEmpty || config.orchestrator != nil else {
            throw ServtermError.importFailed("the file names no server and no orchestrator")
        }
        return Result(config: config, tokens: tokens)
    }

    /// load reads the import file, and then deletes it. It deletes the file
    /// after a failure too, because a damaged file still holds tokens. A
    /// delete that fails is itself a failure, because the tokens stay on
    /// the disk.
    public static func load(from url: URL, files: any FileAccess) -> BootstrapOutcome {
        guard files.exists(at: url) else { return .none }
        var parsed: Result?
        var message: String?
        do {
            parsed = try parse(try files.contents(at: url))
        } catch let error as ServtermError {
            message = error.message
        } catch {
            message = "the app cannot read the import file"
        }
        do {
            try files.remove(at: url)
        } catch {
            return .failed("the app cannot delete the import file. It still holds your tokens.")
        }
        if let message { return .failed(message) }
        guard let parsed else { return .failed("the app cannot read the import file") }
        return .imported(parsed)
    }
}
