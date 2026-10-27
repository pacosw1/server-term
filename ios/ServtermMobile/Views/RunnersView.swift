import SwiftUI
import ServtermKit

/// RunnersView shows the CI runners of every server that reports one.
struct RunnersView: View {
    @Environment(AppModel.self) private var model

    private var hosts: [ServerEntry] { model.runnerServers }

    var body: some View {
        ZStack {
            PageBackground()
            if model.config.servers.isEmpty {
                ContentUnavailableView {
                    Label("No servers", systemImage: "play.square.stack")
                } description: {
                    Text("Add a server in Settings.")
                }
            } else if hosts.isEmpty {
                ContentUnavailableView {
                    Label("No runners", systemImage: "play.square.stack")
                } description: {
                    Text("No CI runners right now.")
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
        .navigationDestination(for: RunnerRoute.self) { route in
            RunnerDetailView(server: route.server)
        }
        .navigationDestination(for: JobRoute.self) { route in
            RunnerJobDetailView(server: route.server, job: route.job)
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
