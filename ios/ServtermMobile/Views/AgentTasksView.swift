import SwiftUI
import ServtermKit

/// AgentTasksView shows the whole checklist of one agent, with the done
/// count and the progress. A long checklist is never cut here.
struct AgentTasksView: View {
    let issue: Int
    let tasks: [OrchestratorTask]

    private var done: Int { tasks.filter(\.done).count }

    private var percent: Double? {
        tasks.isEmpty ? nil : Double(done) / Double(tasks.count) * 100
    }

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("\(done) of \(tasks.count) done")
                            .font(.headline)
                            .monospacedDigit()
                            .contentTransition(.numericText())
                        MeterView(label: "Progress", percent: percent)
                    }
                    .card()
                    VStack(alignment: .leading, spacing: 10) {
                        ForEach(tasks) { task in
                            AgentTaskRow(task: task)
                        }
                    }
                    .card()
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle("Checklist #\(issue)")
        .navigationBarTitleDisplayMode(.inline)
    }
}

/// AgentTaskRow is one checklist item. The daemon carries only the text
/// and the done flag, so there is nothing deeper to open.
struct AgentTaskRow: View {
    let task: OrchestratorTask

    var body: some View {
        Label {
            Text(task.text)
                .font(.subheadline)
                .strikethrough(task.done)
                .foregroundStyle(task.done ? Theme.muted : Theme.text)
                .fixedSize(horizontal: false, vertical: true)
        } icon: {
            Image(systemName: task.done ? "checkmark.square.fill" : "square")
                .foregroundStyle(task.done ? Theme.normal : Theme.muted)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(task.text)
        .accessibilityValue(task.done ? "done" : "not done")
    }
}
