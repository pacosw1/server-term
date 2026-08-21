import SwiftUI
import ServtermSSH

/// PhoneKeyCard shows the public key of this phone, with a copy button, so
/// the user can install it on a host. The private key never appears here,
/// and it never leaves the device.
struct PhoneKeyCard: View {
    @State private var identity: SSHIdentity?
    @State private var error: String?
    @State private var copied = false

    private let store = KeychainIdentityStore()

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("The key of this phone").font(.headline)
            if let identity {
                Text(storageNote(identity))
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
                Text(identity.publicKeyLine)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
                    .foregroundStyle(Theme.text)
                    .padding(10)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Theme.raised)
                    .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
                InfoRow("Fingerprint", value: identity.fingerprint)
                    .font(.caption)
                Button(copied ? "Copied" : "Copy the public key") {
                    UIPasteboard.general.string = identity.publicKeyLine
                    copied = true
                }
                .foregroundStyle(Theme.accent)
                .frame(minHeight: Theme.minimumTapTarget)
                Text("Add this line to ~/.ssh/authorized_keys on the host. The app never sends the key anywhere by itself.")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            } else if let error {
                ErrorBanner(message: error)
            } else {
                Text("The app makes the key when you first open a shell.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
        }
        .card()
        .onAppear(perform: load)
    }

    private func load() {
        do {
            identity = try store.loadOrCreate(comment: ShellIdentityBootstrap.comment)
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func storageNote(_ identity: SSHIdentity) -> String {
        switch identity.storage {
        case .secureEnclave:
            return "The private key lives in the Secure Enclave. It cannot be read, copied or moved, even by this app."
        case .keychain:
            return "This device has no Secure Enclave, so the private key lives in the Keychain, readable only while the phone is unlocked and never copied to another device."
        }
    }
}
