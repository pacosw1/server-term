import SwiftUI
import ServtermKit

struct ServerListView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        NavigationStack {
            ZStack {
                PageBackground()
                if model.config.servers.isEmpty {
                    EmptyServerView()
                } else {
                    ScrollView {
                        LazyVStack(spacing: Theme.cardSpacing) {
                            if let message = model.bootstrapMessage {
                                if model.bootstrapFailed {
                                    ErrorBanner(message: message)
                                } else {
                                    Text(message)
                                        .font(.footnote)
                                        .foregroundStyle(.secondary)
                                        .frame(maxWidth: .infinity, alignment: .leading)
                                }
                            }
                            ForEach(model.config.servers) { server in
                                NavigationLink(value: server) {
                                    ServerCardView(
                                        server: server,
                                        reading: model.servers[server.id],
                                        trend: model.cpuTrend(for: server.id),
                                        transport: model.transports[server.id] ?? .idle)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        .padding(.horizontal)
                        .padding(.bottom, Theme.cardSpacing)
                    }
                    .refreshable { await model.refreshAllServers(force: true) }
                }
            }
            .navigationTitle("Servers")
            .navigationDestination(for: ServerEntry.self) { server in
                ServerDetailView(server: server)
            }
        }
        // The list shows every server, so it asks for a socket for each of
        // them. The sockets close when the user leaves the tab.
        .onAppear { model.setLiveWants(Set(model.config.servers.map(\.id)), for: "servers") }
        .onDisappear { model.setLiveWants([], for: "servers") }
        // The poll stays as the fallback. It skips a server that a healthy
        // socket already feeds.
        .pollEvery(seconds: 3) {
            await model.refreshAllServers()
        }
    }
}

/// EmptyServerView tells the user what to do when no server is set up.
struct EmptyServerView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        VStack(spacing: Theme.cardSpacing) {
            if let message = model.bootstrapMessage, model.bootstrapFailed {
                ErrorBanner(message: message).padding(.horizontal)
            }
            ContentUnavailableView {
                Label("No server yet", systemImage: "server.rack")
            } description: {
                Text("Open Settings and add a server. You need its agent URL and its token.")
            }
        }
    }
}
