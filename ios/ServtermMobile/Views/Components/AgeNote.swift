import SwiftUI
import ServtermKit

/// AgeNote says how old the reading is. It turns orange when the last poll
/// failed, so an old value never reads as a fresh one.
struct AgeNote: View {
    let fetchedAt: Date?
    let isStale: Bool

    var body: some View {
        Label {
            Text(text)
        } icon: {
            Image(systemName: isStale ? "clock.badge.exclamationmark" : "clock")
        }
        .font(.caption)
        .foregroundStyle(isStale ? Theme.warning : Theme.muted)
        .accessibilityLabel(text)
    }

    private var text: String {
        guard let fetchedAt else { return "no reading yet" }
        return "read " + Format.relativeAge(fetchedAt)
    }
}
