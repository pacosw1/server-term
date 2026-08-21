import SwiftUI
import ServtermKit

/// AgentsView is the preview list. It holds a compact header, one row for
/// each live agent, and the history under them. A tap opens the full view
/// of one agent. The screen is read only.
struct AgentsView: View {
    @Environment(AppModel.self) private var model

    private var reading: Reading<OrchestratorSnapshot> { model.orchestrator }

    var body: some View {
        NavigationStack {
            ZStack {
                PageBackground()
                if model.config.orchestrator == nil {
                    ContentUnavailableView {
                        Label("No orchestrator yet", systemImage: "cpu")
                    } description: {
                        Text("Open Settings and add the orchestrator endpoint and its token.")
                    }
                } else {
                    ScrollView {
                        LazyVStack(spacing: Theme.cardSpacing) {
                            if let error = reading.error {
                                ErrorBanner(message: error)
                            }
                            if let snapshot = reading.value {
                                AgentsHeaderCard(
                                    snapshot: snapshot, fetchedAt: reading.fetchedAt,
                                    isStale: reading.error != nil)
                                AgentsLiveSection(snapshot: snapshot)
                                AgentsRecentSection(snapshot: snapshot)
                            } else {
                                Text("The app has no snapshot yet.")
                                    .foregroundStyle(Theme.muted)
                                    .card()
                            }
                        }
                        .padding(.horizontal)
                        .padding(.bottom, Theme.cardSpacing)
                    }
                    .refreshable { await model.refreshOrchestrator() }
                }
            }
            .navigationTitle("Agents")
            .navigationDestination(for: OrchestratorAgent.self) { agent in
                AgentDetailView(
                    agent: agent,
                    repo: reading.value?.repo ?? "",
                    costIsEstimate: reading.value?.costIsEstimate ?? true)
            }
        }
        // The orchestrator daemon serves no stream, so this screen polls,
        // and only while it is visible.
        .pollEvery(seconds: 3) {
            await model.refreshOrchestrator()
        }
    }
}
