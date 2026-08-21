import SwiftUI

enum AppTab: Hashable {
    case servers
    case runners
    case agents
    case settings
}

struct RootView: View {
    @State private var selection: AppTab = .servers

    var body: some View {
        TabView(selection: $selection) {
            Tab("Servers", systemImage: "server.rack", value: .servers) {
                ServerListView()
            }
            Tab("Runners", systemImage: "play.square.stack", value: .runners) {
                RunnersView()
            }
            Tab("Agents", systemImage: "cpu", value: .agents) {
                AgentsView()
            }
            Tab("Settings", systemImage: "gearshape", value: .settings) {
                SettingsView()
            }
        }
        // The theme is dark by design, so the app does not follow the
        // appearance of the phone. A light copy of this palette would be a
        // different theme, not the same one.
        .preferredColorScheme(.dark)
        .tint(Theme.accent)
    }
}
