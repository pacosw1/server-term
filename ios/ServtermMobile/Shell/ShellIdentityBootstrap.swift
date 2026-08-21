import Foundation
import ServtermSSH

/// ShellIdentityBootstrap makes the key of this phone at the first start.
/// It writes only the PUBLIC line to a file; the private key stays in the
/// Secure Enclave or the Keychain.
enum ShellIdentityBootstrap {
    /// comment is the last field of the public key line. It names the
    /// device in authorized_keys, so a person can tell the keys apart.
    static let comment = "servterm-mobile@iphone"

    static func prepare() {
        _ = try? KeychainIdentityStore().loadOrCreate(comment: comment)
    }
}
