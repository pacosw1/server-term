import SwiftUI
import ServtermKit

/// AgentTaskSummary is the row that opens the whole checklist.
struct AgentTaskSummary: View {
    let tasks: [OrchestratorTask]

    private var done: Int { tasks.filter(\.done).count }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("Checklist").font(.headline)
                Spacer(minLength: 8)
                Text("\(done) of \(tasks.count)")
                    .font(.subheadline)
                    .monospacedDigit()
                    .foregroundStyle(Theme.muted)
                Image(systemName: "chevron.right")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            MeterView(
                label: "Progress",
                percent: tasks.isEmpty ? nil : Double(done) / Double(tasks.count) * 100,
                cells: BarMeter.compactCells)
            if let next = tasks.first(where: { !$0.done }) {
                Text("next: " + next.text)
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
                    .lineLimit(2)
            }
        }
        .frame(minHeight: Theme.minimumTapTarget)
        .card()
    }
}
