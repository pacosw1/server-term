import SwiftUI
import ServtermKit

struct AgentsLiveSection: View {
    let snapshot: OrchestratorSnapshot

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Live agents").font(.headline)
            if snapshot.agents.isEmpty {
                Text("No agent runs now.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            ForEach(snapshot.agents) { agent in
                NavigationLink(value: agent) {
                    AgentRowView(agent: agent, costIsEstimate: snapshot.costIsEstimate)
                }
                .buttonStyle(.plain)
            }
        }
        .card()
    }
}
