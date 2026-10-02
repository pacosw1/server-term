import SwiftUI

/// AppPage is a screen that the home screen opens from its menu.
enum AppPage: Hashable {
    case runners
    case agents
}

/// RootView is one stack with the server list at the bottom of it. The
/// runners and the agents sit behind the menu, and the settings open as a
/// sheet from the gear, so the first screen asks the user nothing.
struct RootView: View {
    @State private var path = NavigationPath()
    @State private var showsSettings = false

    var body: some View {
        NavigationStack(path: $path) {
            ServerListView()
                .toolbar {
                    ToolbarItem(placement: .topBarLeading) {
                        Menu {
                            Button("Runners", systemImage: "play.square.stack") {
                                path.append(AppPage.runners)
                            }
                            Button("Agents", systemImage: "cpu") {
                                path.append(AppPage.agents)
                            }
                        } label: {
                            Image(systemName: "ellipsis.circle")
                                .accessibilityLabel("More")
                        }
                        .accessibilityIdentifier("more-menu")
                    }
                    ToolbarItem(placement: .topBarTrailing) {
                        Button {
                            showsSettings = true
                        } label: {
                            Image(systemName: "gearshape")
                                .accessibilityLabel("Settings")
                        }
                        .accessibilityIdentifier("open-settings")
                    }
                }
                .navigationDestination(for: AppPage.self) { page in
                    switch page {
                    case .runners: RunnersView()
                    case .agents: AgentsView()
                    }
                }
        }
        .sheet(isPresented: $showsSettings) {
            SettingsView()
                .preferredColorScheme(.dark)
                .tint(Theme.accent)
        }
        // The theme is dark by design, so the app does not follow the
        // appearance of the phone.
        .preferredColorScheme(.dark)
        .tint(Theme.accent)
    }
}
