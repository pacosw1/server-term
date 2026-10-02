import SwiftUI
import ServtermKit

struct ServerListView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        ZStack {
            PageBackground()
            if model.config.servers.isEmpty {
                EmptyServerView()
            } else {
                ScrollView {
                    LazyVStack(spacing: 8) {
                        if let message = model.bootstrapMessage, model.bootstrapFailed {
                            ErrorBanner(message: message)
                        }
                        ForEach(model.config.servers) { server in
                            NavigationLink(value: server) {
                                ServerCardView(server: server, reading: model.servers[server.id])
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
        // The list shows every server, so it asks for a socket for each of
        // them. The sockets close when the user leaves the screen.
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
                Label("No servers", systemImage: "server.rack")
            } description: {
                Text("Tap the gear to add one.")
            }
        }
    }
}
