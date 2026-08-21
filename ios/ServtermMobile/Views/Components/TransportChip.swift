import SwiftUI

/// TransportChip says where the reading comes from. A live socket and a
/// poll are not the same thing, so the screen never hides which one feeds
/// it. The reason for a poll is always visible to the reader.
struct TransportChip: View {
    let transport: Transport

    var body: some View {
        switch transport {
        case .live:
            StateChip(text: "LIVE", color: Theme.normal, systemImage: "dot.radiowaves.left.and.right")
        case .polling:
            StateChip(text: "polling", color: Theme.warning, systemImage: "arrow.triangle.2.circlepath")
        case .idle:
            StateChip(text: "idle", color: Theme.muted, systemImage: "pause")
        }
    }
}
