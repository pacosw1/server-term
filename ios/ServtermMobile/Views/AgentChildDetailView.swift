import SwiftUI
import ServtermKit

/// AgentChildDetailView shows one spark subagent. It shows the exit code
/// only when the daemon sends one, because a missing code is not a zero.
struct AgentChildDetailView: View {
    let child: OrchestratorChild

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    VStack(alignment: .leading, spacing: 10) {
                        HStack(alignment: .top) {
                            Text(child.model.isEmpty ? child.id : child.model)
                                .font(.headline)
                            Spacer(minLength: 8)
                            StateChip(
                                text: child.state, color: AgentState.color(child.state),
                                systemImage: AgentState.icon(child.state))
                        }
                        InfoRow("Identifier", value: child.id.isEmpty ? Format.unknown : child.id)
                        InfoRow("Model", value: child.model.isEmpty ? Format.unknown : child.model)
                        InfoRow("Elapsed", value: Format.duration(seconds: Double(child.elapsedSeconds)))
                        InfoRow("Input tokens", value: "\(child.inputTokens)")
                        InfoRow("Output tokens", value: "\(child.outputTokens)")
                        InfoRow("Process", value: child.pid.map { "\($0)" } ?? Format.unknown)
                        if let exitCode = child.exitCode {
                            InfoRow("Exit code", value: "\(exitCode)")
                        } else {
                            Text("The daemon reports no exit code yet, so this subagent has not finished.")
                                .font(.caption)
                                .foregroundStyle(Theme.muted)
                        }
                    }
                    .font(.subheadline)
                    .card()
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Task").font(.headline)
                        Text(child.task.isEmpty ? "The daemon reports no task text." : child.task)
                            .font(.subheadline)
                            .foregroundStyle(child.task.isEmpty ? Theme.muted : Theme.text)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    .card()
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle("Subagent")
        .navigationBarTitleDisplayMode(.inline)
    }
}
