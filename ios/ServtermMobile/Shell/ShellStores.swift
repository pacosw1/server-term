import Foundation
import Security
import ServtermSSH

/// KeychainFingerprintStore pins one host key for each host. A fingerprint
/// is not a secret, but it must not be editable by anything but the app,
/// so it lives in the Keychain beside the tokens.
struct KeychainFingerprintStore: FingerprintStore {
    private let service = "com.servterm.mobile.hostkeys"

    private func query(_ host: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: host,
        ]
    }

    func fingerprint(forHost host: String) -> String? {
        var request = query(host)
        request[kSecReturnData as String] = true
        request[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(request as CFDictionary, &item) == errSecSuccess,
            let data = item as? Data
        else { return nil }
        return String(data: data, encoding: .utf8)
    }

    func setFingerprint(_ fingerprint: String, forHost host: String) {
        removeFingerprint(forHost: host)
        var request = query(host)
        request[kSecValueData as String] = Data(fingerprint.utf8)
        request[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        SecItemAdd(request as CFDictionary, nil)
    }

    func removeFingerprint(forHost host: String) {
        SecItemDelete(query(host) as CFDictionary)
    }
}

/// KeychainIdentityStore keeps the key of this phone. The Secure Enclave
/// blob it stores is useless on any other device, and a software key is
/// held only when the device has no enclave. The item is readable only
/// while the phone is unlocked, and it never leaves this device.
struct KeychainIdentityStore {
    private let service = "com.servterm.mobile.sshkey"
    private let account = "phone-identity"
    private let storageKey = "servterm.ssh.storage"
    private let commentKey = "servterm.ssh.comment"

    private let defaults: UserDefaults

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    /// load reads the key back, or makes one the first time.
    func loadOrCreate(comment: String) throws -> SSHIdentity {
        if let identity = try load(comment: comment) {
            writePublicKeyFile(identity)
            return identity
        }
        let identity = try SSHIdentity.generate(comment: comment)
        try save(identity)
        writePublicKeyFile(identity)
        return identity
    }

    /// writePublicKeyFile drops the PUBLIC line into the Documents folder,
    /// so the user can pick it up with the Files app. A public key is safe
    /// to copy; the private key never leaves the Keychain or the enclave.
    private func writePublicKeyFile(_ identity: SSHIdentity) {
        guard let folder = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
        else { return }
        let url = folder.appendingPathComponent("servterm-public-key.txt")
        try? Data((identity.publicKeyLine + "\n").utf8).write(to: url, options: .atomic)
    }

    /// load reads the stored key. The comment is not part of the key, so
    /// the app always uses the current one: a renamed phone then shows the
    /// new name without making a new key.
    func load(comment: String) throws -> SSHIdentity? {
        var request: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var item: CFTypeRef?
        guard SecItemCopyMatching(request as CFDictionary, &item) == errSecSuccess,
            let data = item as? Data
        else { return nil }
        request.removeAll()
        let storage: SSHIdentity.Storage =
            defaults.string(forKey: storageKey) == "keychain" ? .keychain : .secureEnclave
        return try SSHIdentity.restore(from: data, storage: storage, comment: comment)
    }

    func save(_ identity: SSHIdentity) throws {
        SecItemDelete(
            [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: service,
                kSecAttrAccount as String: account,
            ] as CFDictionary)
        let request: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: identity.storedData,
            // The key is readable only while the phone is unlocked, and it
            // never moves to another device.
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
        ]
        let status = SecItemAdd(request as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw ShellError.keychain("the app cannot store the key of this phone")
        }
        defaults.set(identity.storage == .keychain ? "keychain" : "enclave", forKey: storageKey)
        defaults.set(identity.comment, forKey: commentKey)
    }
}

enum ShellError: Error, LocalizedError {
    case keychain(String)
    case noUser

    var errorDescription: String? {
        switch self {
        case .keychain(let message): return message
        case .noUser: return "This server has no shell account yet. Set one in Settings."
        }
    }
}
