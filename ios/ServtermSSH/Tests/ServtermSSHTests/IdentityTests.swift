import Foundation
import Testing
@testable import ServtermSSH

@Suite("The phone key")
struct IdentityTests {
    @Test("a generated key gives an OpenSSH public line that a host accepts")
    func publicKeyLine() throws {
        let identity = try SSHIdentity.generateInMemory(comment: "servterm-mobile")
        let line = identity.publicKeyLine
        let parts = line.split(separator: " ")
        #expect(parts.count == 3)
        #expect(parts[0] == "ecdsa-sha2-nistp256" || parts[0] == "ssh-ed25519")
        #expect(parts[2] == "servterm-mobile")
        let blob = try #require(Data(base64Encoded: String(parts[1])))
        #expect(blob.count > 32)
    }

    @Test("the public line carries the same algorithm inside its blob")
    func blobMatchesAlgorithm() throws {
        let identity = try SSHIdentity.generateInMemory(comment: "servterm-mobile")
        let parts = identity.publicKeyLine.split(separator: " ")
        let blob = try #require(Data(base64Encoded: String(parts[1])))
        let name = String(parts[0])
        // An SSH blob starts with the length of the algorithm name and then
        // the name itself.
        let nameBytes = Array(blob.dropFirst(4).prefix(name.utf8.count))
        #expect(String(decoding: nameBytes, as: UTF8.self) == name)
    }

    @Test("two keys differ, so no build ships a fixed key")
    func keysAreUnique() throws {
        let first = try SSHIdentity.generateInMemory(comment: "a")
        let second = try SSHIdentity.generateInMemory(comment: "a")
        #expect(first.publicKeyLine != second.publicKeyLine)
    }

    @Test("a comment with a space cannot break the line")
    func safeComment() throws {
        let identity = try SSHIdentity.generateInMemory(comment: "my phone name")
        #expect(identity.publicKeyLine.split(separator: " ").count == 3)
        #expect(identity.publicKeyLine.hasSuffix("my-phone-name"))
    }

    @Test("the fingerprint reads like the one ssh-keygen prints")
    func fingerprintFormat() throws {
        let identity = try SSHIdentity.generateInMemory(comment: "a")
        #expect(identity.fingerprint.hasPrefix("SHA256:"))
        #expect(identity.fingerprint.contains("=") == false)
    }
}
