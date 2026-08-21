import SwiftUI
import ServtermKit

/// RunnersView shows the CI runners of every server that reports one.
struct RunnersView: View {
    @Environment(AppModel.self) private var model

    private var hosts: [ServerEntry] { model.runnerServers }

    var body: some View {
        NavigationStack {
            ZStack {
                PageBackground()
                if model.config.servers.isEmpty {
                    ContentUnavailableView {
                        Label("No server yet", systemImage: "play.square.stack")
                    } description: {
                        Text("Open Settings and add a server. The runners come from the same agent.")
                    }
                } else if hosts.isEmpty {
                    ContentUnavailableView {
                        Label("No runner", systemImage: "play.square.stack")
                    } description: {
                        Text("No server reports a CI runner now. This is a normal state, not a fault.")
                    }
                } else {
                    ScrollView {
                        LazyVStack(spacing: Theme.cardSpacing) {
                            ForEach(hosts) { server in
                                RunnerHostCard(server: server, reading: model.servers[server.id])
                            }
                        }
                        .padding(.horizontal)
                        .padding(.bottom, Theme.cardSpacing)
                    }
                    .refreshable { await model.refreshAllServers(force: true) }
                }
            }
            .navigationTitle("Runners")
        }
        // The runners screen shows the same servers, so it keeps the same
        // sockets open while it is visible.
        .onAppear { model.setLiveWants(Set(model.config.servers.map(\.id)), for: "runners") }
        .onDisappear { model.setLiveWants([], for: "runners") }
        .pollEvery(seconds: 3) {
            await model.refreshAllServers()
        }
    }
}
