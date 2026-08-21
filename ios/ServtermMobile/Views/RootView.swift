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
    }
}
