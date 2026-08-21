import SwiftUI
import ServtermKit
import ServtermSSH

/// TrustedHostsCard lists the host keys that the app pinned, and lets the
/// user drop one. Dropping a pin is the only way past a changed key, and
/// it is a deliberate act in the settings, never a button on the warning.
struct TrustedHostsCard: View {
    @Environment(AppModel.self) private var model
    @State private var refreshed = 0

    private let store = KeychainFingerprintStore()

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Trusted host keys").font(.headline)
            let hosts = model.config.servers.map(\.host).filter { !$0.isEmpty }
            if hosts.allSatisfy({ store.fingerprint(forHost: $0) == nil }) {
                Text("The app has trusted no host key yet. It pins the key the first time it opens a shell.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(hosts, id: \.self) { host in
                if let fingerprint = store.fingerprint(forHost: host) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(host).font(.subheadline)
                        Text(fingerprint)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(Theme.muted)
                            .textSelection(.enabled)
                        Button("Forget this key") {
                            store.removeFingerprint(forHost: host)
                            refreshed += 1
                        }
                        .font(.caption)
                        .foregroundStyle(Theme.critical)
                        .frame(minHeight: Theme.minimumTapTarget, alignment: .leading)
                    }
                    .padding(.vertical, 6)
                }
            }
            Text("Forget a key only when you know why the host key changed, for example after you rebuilt the machine.")
                .font(.caption)
                .foregroundStyle(Theme.muted)
                .fixedSize(horizontal: false, vertical: true)
        }
        .id(refreshed)
        .card()
    }
}
