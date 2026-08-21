import SwiftUI
import ServtermKit

struct ServerListView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        NavigationStack {
            Group {
                if model.config.servers.isEmpty {
                    EmptyHint(
                        title: "No server yet",
                        message: "Open Settings and add a server. You need its agent URL and its token.")
                } else {
                    List {
                        ForEach(model.config.servers) { server in
                            Section {
                                NavigationLink {
                                    ServerDetailView(server: server)
                                } label: {
                                    ServerRow(server: server, reading: model.servers[server.id])
                                }
                                if let error = model.servers[server.id]?.error {
                                    ErrorBanner(message: error)
                                }
                            }
                        }
                    }
                }
            }
            .navigationTitle("Servers")
        }
        .pollEvery(seconds: 3) {
            await model.refreshAllServers()
        }
    }
}

/// ServerRow shows one server. It shows a value only when the app has a
/// reading. It shows the dash for every value that it does not know.
struct ServerRow: View {
    let server: ServerEntry
    let reading: Reading<Sample>?

    private var sample: Sample? { reading?.value }
    private var isStale: Bool { reading?.error != nil && reading?.value != nil }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(server.name).font(.headline)
                    if !server.location.isEmpty {
                        Text(server.location).font(.caption).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                StateBadge(text: stateText, color: stateColor)
            }
            HStack(spacing: 16) {
                metric("CPU", Format.optionalPercent(sample.map(\.cpuPercent)))
                metric("MEM", Format.optionalPercent(sample?.memoryPercent))
                metric("DISK", Format.optionalPercent(sample?.primaryDisk?.usedPercent))
            }
            AgeNote(fetchedAt: reading?.fetchedAt, isStale: isStale)
        }
        .padding(.vertical, 2)
    }

    private func metric(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label).font(.caption2).foregroundStyle(.secondary)
            Text(value).font(.subheadline.weight(.medium)).monospacedDigit()
        }
    }

    private var stateText: String {
        if reading?.error != nil { return "error" }
        guard let sample else { return reading?.isLoading == true ? "reading" : "unknown" }
        return sample.online ? "online" : "offline"
    }

    private var stateColor: Color {
        switch stateText {
        case "online": return .green
        case "error", "offline": return .red
        default: return .secondary
        }
    }
}
