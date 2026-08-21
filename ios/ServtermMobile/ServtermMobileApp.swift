import SwiftUI

@main
struct ServtermMobileApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
        }
    }
}

struct RootView: View {
    var body: some View {
        TabView {
            ServerListView()
                .tabItem { Label("Servers", systemImage: "server.rack") }
            AgentsView()
                .tabItem { Label("Agents", systemImage: "cpu") }
            SettingsView()
                .tabItem { Label("Settings", systemImage: "gearshape") }
        }
    }
}
