import Crypto
import Foundation
import NIOSSH

/// SSHIdentity is the key that this phone signs with. The private key
/// lives in the Secure Enclave when the device has one, so the bytes never
/// exist outside the chip. On a device without an enclave, for example the
/// simulator, the key is a P-256 key held only in the Keychain.
///
/// The app never writes a private key to a file or to UserDefaults, and it
/// never asks for a password to store.
public struct SSHIdentity: Sendable {
    /// Storage keeps the two kinds apart, because only one of them ever
    /// holds key bytes.
    public enum Storage: Sendable, Equatable {
        case secureEnclave
        case keychain
    }

    public let storage: Storage
    public let comment: String
    private let key: SSHPrivateKeyBox

    /// nioKey is the key that the SSH client signs with.
    public var nioKey: NIOSSHPrivateKey {
        switch key {
        case .enclave(let key): return NIOSSHPrivateKey(secureEnclaveP256Key: key)
        case .software(let key): return NIOSSHPrivateKey(p256Key: key)
        }
    }

    /// publicKeyLine is the line for authorized_keys.
    public var publicKeyLine: String {
        String(openSSHPublicKey: nioKey.publicKey) + " " + Self.safeComment(comment)
    }

    /// fingerprint reads like the figure that ssh-keygen -l prints.
    public var fingerprint: String {
        SSHFingerprint.of(nioKey.publicKey) ?? "SHA256:unknown"
    }

    /// The bytes that the app stores. An enclave key stores an opaque blob
    /// that only this chip can use; a software key stores its raw key.
    public var storedData: Data {
        switch key {
        case .enclave(let key): return key.dataRepresentation
        case .software(let key): return key.rawRepresentation
        }
    }

    init(storage: Storage, comment: String, key: SSHPrivateKeyBox) {
        self.storage = storage
        self.comment = comment
        self.key = key
    }

    /// generate makes a new key. It uses the Secure Enclave when the
    /// device has one.
    public static func generate(comment: String) throws -> SSHIdentity {
        if SecureEnclave.isAvailable {
            let key = try SecureEnclave.P256.Signing.PrivateKey()
            return SSHIdentity(storage: .secureEnclave, comment: comment, key: .enclave(key))
        }
        return try generateInMemory(comment: comment)
    }

    /// generateInMemory makes a software key. The simulator and the tests
    /// use it, because they have no Secure Enclave.
    public static func generateInMemory(comment: String) throws -> SSHIdentity {
        SSHIdentity(
            storage: .keychain, comment: comment, key: .software(P256.Signing.PrivateKey()))
    }

    /// restore reads a key back from its stored bytes.
    public static func restore(from data: Data, storage: Storage, comment: String) throws -> SSHIdentity {
        switch storage {
        case .secureEnclave:
            let key = try SecureEnclave.P256.Signing.PrivateKey(dataRepresentation: data)
            return SSHIdentity(storage: .secureEnclave, comment: comment, key: .enclave(key))
        case .keychain:
            let key = try P256.Signing.PrivateKey(rawRepresentation: data)
            return SSHIdentity(storage: .keychain, comment: comment, key: .software(key))
        }
    }

    /// safeComment keeps the public line to three fields, so a comment can
    /// never split it.
    static func safeComment(_ comment: String) -> String {
        let cleaned = comment.trimmingCharacters(in: .whitespacesAndNewlines)
            .replacingOccurrences(of: " ", with: "-")
        return cleaned.isEmpty ? "servterm-mobile" : cleaned
    }
}

/// SSHPrivateKeyBox holds one of the two key kinds.
enum SSHPrivateKeyBox: @unchecked Sendable {
    case enclave(SecureEnclave.P256.Signing.PrivateKey)
    case software(P256.Signing.PrivateKey)
}

/// SSHFingerprint reads the standard SHA-256 fingerprint of a public key,
/// through the public OpenSSH string. It needs no private API.
public enum SSHFingerprint {
    public static func of(_ key: NIOSSHPublicKey) -> String? {
        of(openSSHLine: String(openSSHPublicKey: key))
    }

    public static func of(openSSHLine line: String) -> String? {
        let parts = line.split(separator: " ")
        guard parts.count >= 2, let blob = Data(base64Encoded: String(parts[1])) else { return nil }
        let digest = Data(SHA256.hash(data: blob)).base64EncodedString()
            .trimmingCharacters(in: CharacterSet(charactersIn: "="))
        return "SHA256:" + digest
    }
}
