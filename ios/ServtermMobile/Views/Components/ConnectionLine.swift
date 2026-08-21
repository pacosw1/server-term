import SwiftUI
import ServtermKit

/// ConnectionLine reports the health of one connection: the transport, the
/// age of the last reading, and the time of the last request. An unknown
/// figure shows a dash.
struct ConnectionLine: View {
    let transport: Transport
    let fetchedAt: Date?
    let roundTrip: TimeInterval?

    var body: some View {
        HStack(spacing: 8) {
            TransportChip(transport: transport)
            Text(ageText)
            Text("·")
            Text(roundTripText)
        }
        .font(.caption)
        .monospacedDigit()
        .foregroundStyle(Theme.muted)
        .accessibilityElement(children: .combine)
    }

    private var ageText: String {
        guard let fetchedAt else { return "no reading" }
        return Format.relativeAge(fetchedAt)
    }

    private var roundTripText: String {
        guard let roundTrip else { return "no request yet" }
        return String(format: "%.0f ms round trip", roundTrip * 1000)
    }
}
