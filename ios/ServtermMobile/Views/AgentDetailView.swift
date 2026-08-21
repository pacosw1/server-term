import SwiftUI
import ServtermKit

/// AgentDetailView is the full view of one agent. It is read only.
struct AgentDetailView: View {
    @Environment(AppModel.self) private var model
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
                    AgentActivityCard(tail: model.activityTail(for: agent.issue), now: Date())
                    tasksSection
                    childrenSection
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
        }
        .navigationTitle("#\(agent.issue)")
        .navigationBarTitleDisplayMode(.inline)
        .navigationDestination(for: OrchestratorChild.self) { child in
            AgentChildDetailView(child: child)
        }
    }

    /// The checklist and the subagents each carry two different empty
    /// states. A nil list means the daemon does not report that kind yet.
    /// An empty list means the agent truly has none. The screen says which.
    @ViewBuilder private var tasksSection: some View {
        switch ReportedList.of(agent.tasks) {
        case .items(let tasks):
            NavigationLink {
                AgentTasksView(issue: agent.issue, tasks: tasks)
            } label: {
                AgentTaskSummary(tasks: tasks)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("task-summary")
        case .notReported, .none:
            Text(ReportedList.of(agent.tasks).message(for: "task"))
                .font(.footnote)
                .foregroundStyle(Theme.muted)
                .card()
        }
    }

    @ViewBuilder private var childrenSection: some View {
        switch ReportedList.of(agent.children) {
        case .items(let children):
            AgentChildrenCard(children: children, agent: agent)
        case .notReported, .none:
            Text(ReportedList.of(agent.children).message(for: "subagent"))
                .font(.footnote)
                .foregroundStyle(Theme.muted)
                .card()
        }
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
