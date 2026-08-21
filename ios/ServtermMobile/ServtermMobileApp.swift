import SwiftUI

@main
struct ServtermMobileApp: App {
    @State private var model = AppModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
                // The app looks for the one-time import file at each start
                // and at each return to the screen.
                .onAppear { model.runBootstrapImport() }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active {
                        model.runBootstrapImport()
                        model.resumeLive()
                    } else {
                        // A phone in a pocket must hold no socket open.
                        model.suspendLive()
                    }
                }
        }
    }
}
