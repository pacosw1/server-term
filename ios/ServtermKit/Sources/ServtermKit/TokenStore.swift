import Foundation
import Security

/// TokenStore keeps the bearer tokens. The app never writes a token into
/// its source, into UserDefaults, or into a log.
public protocol TokenStore: Sendable {
    func token(for id: String) -> String?
    func setToken(_ token: String, for id: String)
    func removeToken(for id: String)
}

/// KeychainTokenStore stores each token as a generic password item. The item
/// is available only after the first unlock of the device, and it never
/// leaves the device.
public struct KeychainTokenStore: TokenStore {
    private let service: String

    public init(service: String = "com.servterm.mobile.tokens") {
        self.service = service
    }

    private func baseQuery(_ id: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: id,
        ]
    }

    public func token(for id: String) -> String? {
        var query = baseQuery(id)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
            let data = item as? Data, let text = String(data: data, encoding: .utf8)
        else { return nil }
        return text
    }

    public func setToken(_ token: String, for id: String) {
        removeToken(for: id)
        guard !token.isEmpty else { return }
        var query = baseQuery(id)
        query[kSecValueData as String] = Data(token.utf8)
        query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        SecItemAdd(query as CFDictionary, nil)
    }

    public func removeToken(for id: String) {
        SecItemDelete(baseQuery(id) as CFDictionary)
    }
}

/// MemoryTokenStore holds tokens only in memory. The tests and the previews
/// use it, so no test touches the real Keychain.
public final class MemoryTokenStore: TokenStore, @unchecked Sendable {
    private let lock = NSLock()
    private var tokens: [String: String]

    public init(tokens: [String: String] = [:]) {
        self.tokens = tokens
    }

    public func token(for id: String) -> String? {
        lock.withLock { tokens[id] }
    }

    public func setToken(_ token: String, for id: String) {
        lock.withLock { tokens[id] = token.isEmpty ? nil : token }
    }

    public func removeToken(for id: String) {
        lock.withLock { tokens[id] = nil }
    }
}
