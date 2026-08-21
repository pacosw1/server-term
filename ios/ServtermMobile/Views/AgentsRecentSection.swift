import SwiftUI
import ServtermKit

struct AgentsRecentSection: View {
    let snapshot: OrchestratorSnapshot

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Recent").font(.headline)
            if snapshot.recent.isEmpty {
                Text("No task finished yet.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            ForEach(snapshot.recent) { item in
                VStack(alignment: .leading, spacing: 5) {
                    HStack(alignment: .firstTextBaseline) {
                        Text("#\(item.issue)")
                            .font(.subheadline)
                            .bold()
                            .monospacedDigit()
                        Spacer(minLength: 8)
                        StateChip(text: item.state, color: AgentState.color(item.state),
                                  systemImage: AgentState.icon(item.state))
                    }
                    Text(item.displayTitle)
                        .font(.subheadline)
                        .lineLimit(2)
                    HStack(spacing: 10) {
                        Text(Format.money(item.costUsd, isEstimate: snapshot.costIsEstimate))
                        if let pr = item.prNumber {
                            Text("PR \(pr)")
                        }
                    }
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
                    if !item.lastError.isEmpty {
                        Text(item.lastError)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                }
                .padding(.vertical, 8)
            }
        }
        .card()
    }
}
