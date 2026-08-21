import Foundation

public struct ProbeOutcome: Sendable, Equatable {
    public let reachable: Bool
    public let detail: String
}

public enum ProbeKind: String, Sendable, Equatable {
    case agent
    case orchestrator

    var port: Int {
        switch self {
        case .agent: return 7843
        case .orchestrator: return 7844
        }
    }

    var path: String {
        switch self {
        case .agent: return "/v1/status"
        case .orchestrator: return "/api/status"
        }
    }
}

public struct ProbeResult: Sendable, Equatable, Identifiable {
    public let host: String
    public let port: Int
    public let kind: ProbeKind
    public let reachable: Bool
    public let detail: String

    public var id: String { "\(host):\(port)" }
    public var url: String { "http://\(host):\(port)" }
}

/// Prober tests the standard servterm ports on one host. The app cannot ask
/// the Tailscale control plane for the host list, so the user gives the host
/// and the app only says which port answers.
public struct Prober: Sendable {
    private let api: ServtermAPI

    public init(api: ServtermAPI = ServtermAPI()) {
        self.api = api
    }

    public func probe(host: String) async -> [ProbeResult] {
        guard let clean = Self.cleanHost(host) else { return [] }
        var results: [ProbeResult] = []
        for kind in [ProbeKind.agent, .orchestrator] {
            let base = "http://\(clean):\(kind.port)"
            let outcome = await api.reach(base: base, path: kind.path)
            results.append(
                ProbeResult(
                    host: clean, port: kind.port, kind: kind,
                    reachable: outcome.reachable, detail: outcome.detail))
        }
        return results
    }

    /// cleanHost takes a pasted address and keeps only the host part.
    public static func cleanHost(_ text: String) -> String? {
        var value = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if value.isEmpty { return nil }
        if let url = URL(string: value), let host = url.host, url.scheme != nil {
            return host
        }
        if let slash = value.firstIndex(of: "/") { value = String(value[value.startIndex..<slash]) }
        if let colon = value.firstIndex(of: ":") { value = String(value[value.startIndex..<colon]) }
        return value.isEmpty ? nil : value
    }
}
