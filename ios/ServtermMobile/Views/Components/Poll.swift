import SwiftUI

extension View {
    /// pollEvery repeats an action while the screen is visible. SwiftUI
    /// cancels the task when the screen goes away, so the app stops reading.
    func pollEvery(seconds: Double = 3, action: @escaping @MainActor () async -> Void) -> some View {
        task {
            while !Task.isCancelled {
                await action()
                try? await Task.sleep(for: .seconds(seconds))
            }
        }
    }
}
