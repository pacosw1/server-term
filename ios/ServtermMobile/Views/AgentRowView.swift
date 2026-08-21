import SwiftUI
import ServtermKit

/// AgentRowView is the preview of one agent. A tap opens the full view.
struct AgentRowView: View {
    let agent: OrchestratorAgent
    let costIsEstimate: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .firstTextBaseline) {
                    Text("#\(agent.issue)")
                        .font(.subheadline)
                        .bold()
                        .monospacedDigit()
                    Spacer(minLength: 8)
                    StateChip(text: agent.state, color: AgentState.color(agent.state),
                              systemImage: AgentState.icon(agent.state))
                }
                Text(agent.displayTitle)
                    .font(.subheadline)
                    .lineLimit(2)
                    .multilineTextAlignment(.leading)
                HStack(spacing: 10) {
                    Label(Format.duration(seconds: Double(agent.elapsedSeconds)), systemImage: "clock")
                    Label(
                        Format.money(agent.costUsd, isEstimate: costIsEstimate),
                        systemImage: "dollarsign.circle")
                    Label(Format.percent(agent.cpuPercent), systemImage: "cpu")
                }
                .font(.caption)
                .monospacedDigit()
                .foregroundStyle(.secondary)
            }
            Image(systemName: "chevron.right")
                .font(.footnote)
                .foregroundStyle(.tertiary)
                .padding(.top, 4)
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.quaternary.opacity(0.35), in: .rect(cornerRadius: 12))
        .contentShape(.rect)
    }
}

/// AgentState maps a state word to one colour and one symbol, so the state
/// is readable without colour.
enum AgentState {
    static func color(_ state: String) -> Color {
        switch state {
        case "working", "running", "live": return Theme.normal
        case "blocked", "failed", "error": return Theme.critical
        case "done", "merged": return Theme.accent
        default: return .secondary
        }
    }

    static func icon(_ state: String) -> String {
        switch state {
        case "working", "running", "live": return "bolt.fill"
        case "blocked", "failed", "error": return "exclamationmark.triangle.fill"
        case "done", "merged": return "checkmark.circle.fill"
        default: return "circle"
        }
    }
}
