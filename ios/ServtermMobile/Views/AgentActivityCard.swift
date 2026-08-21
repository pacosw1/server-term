import SwiftUI
import ServtermKit

/// AgentActivityCard shows the lines that the agent reported while the app
/// watched. The daemon serves one line for each poll and keeps no history,
/// so this history belongs to the app and starts when the app starts.
struct AgentActivityCard: View {
    let tail: ActivityTail
    let now: Date

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Activity").font(.headline)
            if tail.isEmpty {
                Text("The app has seen no activity line yet. The daemon sends one line for each poll, and the app keeps the last \(ActivityTail.limit).")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            } else {
                Text("The last \(tail.entries.count) lines that the app saw. The history starts when the app opens.")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                ForEach(tail.entries) { entry in
                    VStack(alignment: .leading, spacing: 3) {
                        Text(entry.text)
                            .font(.subheadline)
                            .foregroundStyle(Theme.text)
                            .fixedSize(horizontal: false, vertical: true)
                        Text(Format.relativeAge(entry.at, now: now))
                            .font(.caption)
                            .monospacedDigit()
                            .foregroundStyle(Theme.muted)
                    }
                    .padding(.vertical, 8)
                    .padding(.horizontal, 12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Theme.raised)
                    .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
                    .accessibilityElement(children: .combine)
                }
            }
        }
        .card()
    }
}
