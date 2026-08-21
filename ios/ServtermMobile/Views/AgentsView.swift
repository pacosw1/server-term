import SwiftUI
import ServtermKit

/// AgentsView shows the orchestrator snapshot. The screen is read only: it
/// never changes the mode and never steers an agent.
struct AgentsView: View {
    @Environment(AppModel.self) private var model

    private var reading: Reading<OrchestratorSnapshot> { model.orchestrator }
    private var snapshot: OrchestratorSnapshot? { reading.value }

    var body: some View {
        NavigationStack {
            Group {
                if model.config.orchestrator == nil {
                    EmptyHint(
                        title: "No orchestrator yet",
                        message: "Open Settings and add the orchestrator endpoint and its token.")
                } else {
                    List {
                        if let error = reading.error {
                            ErrorBanner(message: error)
                        }
                        if let snapshot {
                            headerSection(snapshot)
                            budgetSection(snapshot)
                            totalsSection(snapshot)
                            if let limits = snapshot.limits { limitsSection(limits) }
                            if let disk = snapshot.disk { diskSection(disk) }
                            liveSection(snapshot)
                            recentSection(snapshot)
                        } else {
                            Text("The app has no snapshot yet.").foregroundStyle(.secondary)
                        }
                    }
                }
            }
            .navigationTitle("Agents")
        }
        .pollEvery(seconds: 3) {
            await model.refreshOrchestrator()
        }
    }

    private func headerSection(_ snapshot: OrchestratorSnapshot) -> some View {
        Section("Daemon") {
            HStack {
                Text("Mode")
                Spacer()
                StateBadge(text: snapshot.mode.isEmpty ? "unknown" : snapshot.mode, color: modeColor(snapshot.mode))
            }
            MetricRow(label: "Health", value: snapshot.healthy ? "healthy" : "not healthy")
            MetricRow(label: "Repository", value: snapshot.repo.isEmpty ? Format.unknown : snapshot.repo)
            MetricRow(label: "Account", value: snapshot.accountLabel)
            MetricRow(label: "CPU", value: Format.percent(snapshot.daemon.cpuPercent))
            MetricRow(label: "Memory", value: Format.bytes(snapshot.daemon.rssBytes))
            MetricRow(label: "Uptime", value: Format.duration(seconds: Double(snapshot.daemon.uptimeSeconds)))
            AgeNote(fetchedAt: reading.fetchedAt, isStale: reading.error != nil)
        }
    }

    private func budgetSection(_ snapshot: OrchestratorSnapshot) -> some View {
        Section("Budget") {
            MetricRow(label: "Day", value: snapshot.costText)
            MetricRow(
                label: "Hour",
                value: Format.money(snapshot.budget.hourUsd, isEstimate: snapshot.costIsEstimate)
                    + " of " + Format.money(snapshot.budget.hourLimitUsd, isEstimate: false))
            MetricRow(
                label: "Week",
                value: Format.money(snapshot.budget.weekUsd, isEstimate: snapshot.costIsEstimate)
                    + " of " + Format.money(snapshot.budget.weekLimitUsd, isEstimate: false))
            MetricRow(
                label: "Day left",
                value: Format.money(snapshot.budget.dayRemainingUsd, isEstimate: snapshot.costIsEstimate))
            if !snapshot.budget.paceNote.isEmpty {
                Text(snapshot.budget.paceNote).font(.footnote).foregroundStyle(.secondary)
            }
            if snapshot.costIsEstimate {
                Text("The plan has no price for each call, so the daemon computes these figures. They are an estimate, not a charge.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func totalsSection(_ snapshot: OrchestratorSnapshot) -> some View {
        Section("Totals") {
            MetricRow(label: "Live", value: "\(snapshot.totals.live)")
            MetricRow(label: "Done", value: "\(snapshot.totals.done)")
            MetricRow(label: "Blocked", value: "\(snapshot.totals.blocked)")
            MetricRow(label: "Input tokens", value: "\(snapshot.totals.inputTokens)")
            MetricRow(label: "Output tokens", value: "\(snapshot.totals.outputTokens)")
            MetricRow(
                label: "Cost",
                value: Format.money(snapshot.totals.costUsd, isEstimate: snapshot.costIsEstimate))
        }
    }

    private func limitsSection(_ limits: OrchestratorLimits) -> some View {
        Section("Plan use") {
            if let weekly = limits.weekly {
                PercentBar(label: "Week", percent: weekly.usedPercent, detail: resetText(weekly.resetsAt))
            }
            if let fiveHour = limits.fiveHour {
                PercentBar(label: "Five hours", percent: fiveHour.usedPercent, detail: resetText(fiveHour.resetsAt))
            }
            if limits.weekly == nil && limits.fiveHour == nil {
                Text("The daemon reports no plan window.").foregroundStyle(.secondary)
            }
        }
    }

    private func diskSection(_ disk: OrchestratorDisk) -> some View {
        Section("Daemon host disk") {
            PercentBar(
                label: "Use", percent: disk.usedPercent,
                detail: "\(Format.bytes(disk.usedBytes)) of \(Format.bytes(disk.totalBytes)) · \(Format.bytes(disk.freeBytes)) free")
        }
    }

    private func liveSection(_ snapshot: OrchestratorSnapshot) -> some View {
        Section("Live agents") {
            if snapshot.agents.isEmpty {
                Text("No agent runs now.").foregroundStyle(.secondary)
            }
            ForEach(snapshot.agents) { agent in
                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text("#\(agent.issue)").font(.subheadline.weight(.semibold))
                        Spacer()
                        StateBadge(text: agent.state, color: stateColor(agent.state))
                    }
                    Text(agent.displayTitle).font(.subheadline)
                    Text(
                        "\(Format.duration(seconds: Double(agent.elapsedSeconds))) · "
                            + Format.money(agent.costUsd, isEstimate: snapshot.costIsEstimate)
                            + " · cycle \(agent.cycle) · \(Format.percent(agent.cpuPercent)) cpu"
                    )
                    .font(.caption).foregroundStyle(.secondary)
                    if let children = agent.children, !children.isEmpty {
                        Text("subagents: \(agent.childrenRunning) run, \(agent.childrenDone) done, \(agent.childrenFailed) failed")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    if !agent.lastError.isEmpty {
                        Text(agent.lastError).font(.caption).foregroundStyle(.red)
                    }
                }
                .padding(.vertical, 2)
            }
        }
    }

    private func recentSection(_ snapshot: OrchestratorSnapshot) -> some View {
        Section("Recent") {
            if snapshot.recent.isEmpty {
                Text("No task finished yet.").foregroundStyle(.secondary)
            }
            ForEach(snapshot.recent) { item in
                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text("#\(item.issue)").font(.subheadline.weight(.semibold))
                        Spacer()
                        StateBadge(text: item.state, color: stateColor(item.state))
                    }
                    Text(item.displayTitle).font(.subheadline)
                    HStack(spacing: 8) {
                        Text(Format.money(item.costUsd, isEstimate: snapshot.costIsEstimate))
                        if let pr = item.prNumber { Text("PR \(pr)") }
                    }
                    .font(.caption).foregroundStyle(.secondary)
                    if !item.lastError.isEmpty {
                        Text(item.lastError).font(.caption).foregroundStyle(.secondary)
                    }
                }
                .padding(.vertical, 2)
            }
        }
    }

    private func resetText(_ epochSeconds: Int64) -> String {
        guard epochSeconds > 0 else { return "" }
        let date = Date(timeIntervalSince1970: TimeInterval(epochSeconds))
        return "resets " + date.formatted(date: .abbreviated, time: .shortened)
    }

    private func modeColor(_ mode: String) -> Color {
        switch mode {
        case "fast": return .green
        case "economy": return .orange
        case "paused": return .secondary
        default: return .secondary
        }
    }

    private func stateColor(_ state: String) -> Color {
        switch state {
        case "working", "running", "live": return .green
        case "blocked", "failed", "error": return .red
        case "done", "merged": return .blue
        default: return .secondary
        }
    }
}
