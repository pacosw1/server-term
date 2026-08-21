import Foundation
import Testing
@testable import ServtermSSH

/// MemoryFingerprintStore keeps the pinned fingerprints in memory, so no
/// test writes to the real store.
final class MemoryFingerprintStore: FingerprintStore, @unchecked Sendable {
    private let lock = NSLock()
    private var values: [String: String]

    init(_ values: [String: String] = [:]) { self.values = values }


    func fingerprint(forHost host: String) -> String? { lock.withLock { values[host] } }

    func setFingerprint(_ fingerprint: String, forHost host: String) {
        lock.withLock { values[host] = fingerprint }
    }

    func removeFingerprint(forHost host: String) { lock.withLock { values[host] = nil } }
}

@Suite("Host keys")
struct HostKeyTests {
    @Test("the first key of a host is trusted and then pinned")
    func trustOnFirstUse() {
        let store = MemoryFingerprintStore()
        let checker = HostKeyChecker(store: store)
        #expect(checker.decide(host: "host-a", fingerprint: "SHA256:aaa") == .firstUse)
        checker.pin(host: "host-a", fingerprint: "SHA256:aaa")
        #expect(checker.decide(host: "host-a", fingerprint: "SHA256:aaa") == .known)
    }

    @Test("a changed key is refused, and the old one is named")
    func changedKey() {
        let store = MemoryFingerprintStore(["host-a": "SHA256:aaa"])
        let checker = HostKeyChecker(store: store)
        let decision = checker.decide(host: "host-a", fingerprint: "SHA256:bbb")
        #expect(decision == .changed(pinned: "SHA256:aaa"))
        #expect(decision.allowsConnection == false)
    }

    @Test("a first use and a known key both allow the connection")
    func allowedDecisions() {
        #expect(HostKeyDecision.firstUse.allowsConnection)
        #expect(HostKeyDecision.known.allowsConnection)
    }

    @Test("a changed key never pins itself, so a retry cannot quietly accept it")
    func changedKeyIsNotPinned() {
        let store = MemoryFingerprintStore(["host-a": "SHA256:aaa"])
        let checker = HostKeyChecker(store: store)
        _ = checker.decide(host: "host-a", fingerprint: "SHA256:bbb")
        #expect(store.fingerprint(forHost: "host-a") == "SHA256:aaa")
    }

    @Test("one host does not lend its key to another host")
    func hostsAreSeparate() {
        let store = MemoryFingerprintStore(["host-a": "SHA256:aaa"])
        let checker = HostKeyChecker(store: store)
        #expect(checker.decide(host: "host-b", fingerprint: "SHA256:aaa") == .firstUse)
    }

    @Test("the warning names both fingerprints, so a person can compare them")
    func warningText() {
        let decision = HostKeyDecision.changed(pinned: "SHA256:aaa")
        let text = decision.warning(host: "host-a", offered: "SHA256:bbb")
        #expect(text.contains("host-a"))
        #expect(text.contains("SHA256:aaa"))
        #expect(text.contains("SHA256:bbb"))
    }

    @Test("forgetting a host clears its pin, for a real key rotation")
    func forget() {
        let store = MemoryFingerprintStore(["host-a": "SHA256:aaa"])
        let checker = HostKeyChecker(store: store)
        checker.forget(host: "host-a")
        #expect(checker.decide(host: "host-a", fingerprint: "SHA256:bbb") == .firstUse)
    }
}
