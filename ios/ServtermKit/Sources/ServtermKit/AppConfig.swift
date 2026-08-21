import Foundation

/// ServerEntry is one servterm agent that the app reads. The token is not
/// here. The app keeps every token in the Keychain, under the entry id.
public struct ServerEntry: Codable, Sendable, Hashable, Identifiable {
    public let id: UUID
    public var name: String
    public var agentURL: String
    public var location: String

    public init(id: UUID = UUID(), name: String, agentURL: String, location: String = "") {
        self.id = id
        self.name = name
        self.agentURL = agentURL
        self.location = location
    }
}

/// OrchestratorEntry is the agent orchestrator daemon.
public struct OrchestratorEntry: Codable, Sendable, Equatable, Identifiable {
    public let id: UUID
    public var name: String
    public var endpoint: String

    public init(id: UUID = UUID(), name: String, endpoint: String) {
        self.id = id
        self.name = name
        self.endpoint = endpoint
    }
}

/// AppConfig is the whole app setup, without any secret.
public struct AppConfig: Codable, Sendable, Equatable {
    public var servers: [ServerEntry] = []
    public var orchestrator: OrchestratorEntry?

    public init(servers: [ServerEntry] = [], orchestrator: OrchestratorEntry? = nil) {
        self.servers = servers
        self.orchestrator = orchestrator
    }

    public mutating func remove(serverID: UUID) {
        servers.removeAll { $0.id == serverID }
    }

    public mutating func upsert(_ server: ServerEntry) {
        if let index = servers.firstIndex(where: { $0.id == server.id }) {
            servers[index] = server
        } else {
            servers.append(server)
        }
    }
}

/// ImportedConfig is the result of reading a pasted servterm config.
public struct ImportedConfig: Sendable, Equatable {
    public var servers: [ServerEntry] = []
    public var orchestrator: OrchestratorEntry?
}

/// ConfigImport reads a pasted servterm config. It accepts the JSON form and
/// the small YAML subset that the real config.yaml uses. It never reads a
/// token: a token_file line names a file on the desktop machine, and the
/// phone cannot open it.
public enum ConfigImport {
    public static func parse(_ text: String) throws -> ImportedConfig {
        let result = parseJSON(text) ?? parseYAML(text)
        guard let result, !result.servers.isEmpty || result.orchestrator != nil else {
            throw ServtermError.importFailed("no server and no orchestrator widget found")
        }
        return result
    }

    private static func parseJSON(_ text: String) -> ImportedConfig? {
        guard let data = text.data(using: .utf8),
            let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return nil }
        var result = ImportedConfig()
        for item in root["servers"] as? [[String: Any]] ?? [] {
            guard let url = item["agent_url"] as? String, !url.isEmpty else { continue }
            result.servers.append(
                ServerEntry(
                    name: item["name"] as? String ?? url,
                    agentURL: url,
                    location: item["location"] as? String ?? ""))
        }
        for item in root["widgets"] as? [[String: Any]] ?? [] {
            guard item["type"] as? String == "orchestrator",
                let endpoint = item["endpoint"] as? String, !endpoint.isEmpty,
                result.orchestrator == nil
            else { continue }
            result.orchestrator = OrchestratorEntry(
                name: item["name"] as? String ?? "orchestrator", endpoint: endpoint)
        }
        return result
    }

    /// parseYAML reads only the shape of the servterm config: a top level
    /// "servers" list and a top level "widgets" list, with plain key and
    /// value lines. It does not support anchors, block text, or nesting.
    private static func parseYAML(_ text: String) -> ImportedConfig? {
        var result = ImportedConfig()
        var section = ""
        var current: [String: String] = [:]

        func flush() {
            defer { current = [:] }
            if section == "servers" {
                guard let url = current["agent_url"], !url.isEmpty else { return }
                result.servers.append(
                    ServerEntry(
                        name: current["name"] ?? url,
                        agentURL: url,
                        location: current["location"] ?? ""))
            }
            if section == "widgets", current["type"] == "orchestrator", result.orchestrator == nil,
                let endpoint = current["endpoint"], !endpoint.isEmpty
            {
                result.orchestrator = OrchestratorEntry(
                    name: current["name"] ?? "orchestrator", endpoint: endpoint)
            }
        }

        for rawLine in text.components(separatedBy: .newlines) {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            if line.isEmpty || line.hasPrefix("#") { continue }
            let indent = rawLine.prefix { $0 == " " }.count
            if indent == 0, line.hasSuffix(":") {
                flush()
                section = String(line.dropLast())
                continue
            }
            if indent == 0 { flush(); section = ""; continue }
            if line.hasPrefix("- ") {
                flush()
                if let pair = keyValue(String(line.dropFirst(2))) { current[pair.0] = pair.1 }
                continue
            }
            if let pair = keyValue(line) { current[pair.0] = pair.1 }
        }
        flush()
        return result
    }

    private static func keyValue(_ line: String) -> (String, String)? {
        guard let colon = line.firstIndex(of: ":") else { return nil }
        let key = String(line[line.startIndex..<colon]).trimmingCharacters(in: .whitespaces)
        var value = String(line[line.index(after: colon)...]).trimmingCharacters(in: .whitespaces)
        if value.hasPrefix("\""), value.hasSuffix("\""), value.count >= 2 {
            value = String(value.dropFirst().dropLast())
        }
        if key.isEmpty { return nil }
        return (key, value)
    }
}
