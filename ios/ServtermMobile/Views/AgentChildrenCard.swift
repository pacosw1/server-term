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
            ForEach(children) { child in
                NavigationLink(value: child) {
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
                    HStack {
                        Spacer()
                        Image(systemName: "chevron.right")
                            .font(.footnote)
                            .foregroundStyle(Theme.muted)
                    }
                }
                .padding(.vertical, 8)
                .padding(.horizontal, 12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.raised)
                .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
                }
                .buttonStyle(.plain)
            }
        }
        .card()
    }
}
