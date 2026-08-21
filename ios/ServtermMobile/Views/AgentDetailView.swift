import SwiftUI
import ServtermKit

/// AgentDetailView is the full view of one agent. It is read only.
struct AgentDetailView: View {
    let agent: OrchestratorAgent
    let repo: String
    let costIsEstimate: Bool

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    identity
                    usage
                    if let tasks = agent.tasks {
                        AgentTaskCard(tasks: tasks)
                    } else {
                        Text("The daemon does not report a checklist for this agent.")
                            .font(.footnote)
                            .foregroundStyle(Theme.muted)
                            .card()
                    }
                    if let children = agent.children {
                        AgentChildrenCard(children: children, agent: agent)
                    } else {
                        Text("The daemon does not report subagents for this agent.")
                            .font(.footnote)
                            .foregroundStyle(Theme.muted)
                            .card()
                    }
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle("#\(agent.issue)")
        .navigationBarTitleDisplayMode(.inline)
    }

    private var identity: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                Text(agent.displayTitle)
                    .font(.headline)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 8)
                StateChip(text: agent.state, color: AgentState.color(agent.state),
                          systemImage: AgentState.icon(agent.state))
            }
            InfoRow("Elapsed", value: Format.duration(seconds: Double(agent.elapsedSeconds)))
            InfoRow("Cycle", value: "\(agent.cycle)")
            InfoRow("Turns", value: "\(agent.turns)")
            InfoRow("Branch", value: agent.branch.isEmpty ? Format.unknown : agent.branch)
            InfoRow("Last activity", value: agent.lastActivity ?? Format.unknown)
            InfoRow(
                "Activity age",
                value: agent.activityAgeSeconds.map { Format.duration(seconds: Double($0)) }
                    ?? Format.unknown)
            if let url = agent.issueURL(repo: repo) {
                Link("Open the issue", destination: url)
            }
            if let url = agent.pullRequestURL(repo: repo) {
                Link("Open the pull request", destination: url)
            }
            if !agent.lastError.isEmpty {
                Text(agent.lastError)
                    .font(.footnote)
                    .foregroundStyle(Theme.critical)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .font(.subheadline)
        .card()
    }

    private var usage: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "CPU", value: Format.percent(agent.cpuPercent),
                    tint: Theme.color(for: agent.cpuPercent), systemImage: "cpu")
                StatTile(
                    label: "Memory", value: Format.bytes(agent.rssBytes),
                    systemImage: "memorychip")
                StatTile(
                    label: "Cost", value: Format.money(agent.costUsd, isEstimate: costIsEstimate),
                    systemImage: "dollarsign.circle")
            }
            Divider()
            InfoRow("Input tokens", value: "\(agent.inputTokens)")
            InfoRow("Output tokens", value: "\(agent.outputTokens)")
            InfoRow("Process", value: agent.pid > 0 ? "\(agent.pid)" : Format.unknown)
            InfoRow("Worktree", value: agent.worktree.isEmpty ? Format.unknown : agent.worktree)
            InfoRow("Worktree disk", value: Format.optionalBytes(agent.worktreeDiskBytes))
            if let weekly = agent.weeklyPercentUsed {
                MeterView(label: "Plan week", percent: weekly)
            }
        }
        .font(.subheadline)
        .card()
    }
}
