import SwiftUI
import ServtermKit

/// AgentTaskCard shows the checklist that the agent keeps for itself.
struct AgentTaskCard: View {
    let tasks: [OrchestratorTask]

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Checklist").font(.headline)
            if tasks.isEmpty {
                Text("The agent keeps no task yet.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            ForEach(tasks) { task in
                Label {
                    Text(task.text)
                        .font(.subheadline)
                        .strikethrough(task.done)
                        .foregroundStyle(task.done ? .secondary : .primary)
                        .fixedSize(horizontal: false, vertical: true)
                } icon: {
                    Image(systemName: task.done ? "checkmark.circle.fill" : "circle")
                        .foregroundStyle(task.done ? Theme.normal : .secondary)
                }
            }
        }
        .card()
    }
}
