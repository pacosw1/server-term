import Foundation

/// FingerprintStore keeps the host key that the app trusted for each host.
/// The app writes it to the Keychain; a test keeps it in memory.
public protocol FingerprintStore: Sendable {
    func fingerprint(forHost host: String) -> String?
    func setFingerprint(_ fingerprint: String, forHost host: String)
    func removeFingerprint(forHost host: String)
}

/// HostKeyDecision is the answer for one host key.
public enum HostKeyDecision: Sendable, Equatable {
    /// firstUse means the app never saw this host. It trusts the key once
    /// and pins it.
    case firstUse
    /// known means the key is the pinned one.
    case known
    /// changed means the host offered a different key. The app refuses the
    /// connection. There is no way to continue past this.
    case changed(pinned: String)

    public var allowsConnection: Bool {
        switch self {
        case .firstUse, .known: return true
        case .changed: return false
        }
    }

    /// warning is the plain text for a changed key. It names both
    /// fingerprints, so a person can compare them with ssh-keygen -l.
    public func warning(host: String, offered: String) -> String {
        guard case .changed(let pinned) = self else { return "" }
        return """
            The key of \(host) changed. The app refuses to connect.

            The app trusted: \(pinned)
            The host offers: \(offered)

            A changed key means the host was rebuilt, or somebody is between \
            you and the host. Check the host key on the machine itself with \
            ssh-keygen -l -f /etc/ssh/ssh_host_ed25519_key.pub. Remove the \
            trusted key in Settings only when you know why it changed.
            """
    }
}

/// HostKeyChecker holds the trust on first use rule. It never accepts a
/// changed key, and it offers no way to skip the warning.
public struct HostKeyChecker: Sendable {
    private let store: any FingerprintStore

    public init(store: any FingerprintStore) {
        self.store = store
    }

    public func decide(host: String, fingerprint: String) -> HostKeyDecision {
        guard let pinned = store.fingerprint(forHost: host) else { return .firstUse }
        return pinned == fingerprint ? .known : .changed(pinned: pinned)
    }

    /// pin stores a key. The caller pins only after a first use, never
    /// after a change.
    public func pin(host: String, fingerprint: String) {
        store.setFingerprint(fingerprint, forHost: host)
    }

    /// forget drops the pin, for a host key that really did rotate. The
    /// user does this by hand, in the settings.
    public func forget(host: String) {
        store.removeFingerprint(forHost: host)
    }
}
