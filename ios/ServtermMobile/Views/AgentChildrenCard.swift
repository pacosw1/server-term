import SwiftUI
import ServtermKit

/// AgentChildrenCard shows the subagents that this agent started.
struct AgentChildrenCard: View {
    let children: [OrchestratorChild]
    let agent: OrchestratorAgent

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Subagents").font(.headline)
            Text("\(agent.childrenRunning) run · \(agent.childrenDone) done · \(agent.childrenFailed) failed")
                .font(.footnote)
                .foregroundStyle(Theme.muted)
            if children.isEmpty {
                Text("This agent started no subagent.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(children) { child in
                VStack(alignment: .leading, spacing: 4) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(child.model.isEmpty ? child.id : child.model)
                            .font(.subheadline)
                            .bold()
                        Spacer(minLength: 8)
                        StateChip(text: child.state, color: AgentState.color(child.state),
                                  systemImage: AgentState.icon(child.state))
                    }
                    Text(child.task)
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .lineLimit(3)
                    Text("\(Format.duration(seconds: Double(child.elapsedSeconds))) · in \(child.inputTokens) · out \(child.outputTokens)")
                        .font(.caption)
                        .monospacedDigit()
                        .foregroundStyle(Theme.muted)
                }
                .padding(.vertical, 6)
            }
        }
        .card()
    }
}
