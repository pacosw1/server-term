import SwiftUI
import ServtermKit

/// AgentsHeaderCard is the compact summary: the mode, the live count, the
/// plan use, and the cost line.
struct AgentsHeaderCard: View {
    let snapshot: OrchestratorSnapshot
    let fetchedAt: Date?
    let isStale: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(snapshot.repo.isEmpty ? "orchestrator" : snapshot.repo)
                        .font(.headline)
                        .lineLimit(1)
                    Text(snapshot.accountLabel)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                Spacer(minLength: 8)
                StateChip(text: modeText, color: modeColor, systemImage: modeIcon)
            }
            HStack(alignment: .top, spacing: 12) {
                StatTile(
                    label: "Live", value: "\(snapshot.totals.live)", tint: Theme.normal,
                    systemImage: "bolt.fill")
                StatTile(label: "Done", value: "\(snapshot.totals.done)", systemImage: "checkmark")
                StatTile(
                    label: "Blocked", value: "\(snapshot.totals.blocked)",
                    tint: snapshot.totals.blocked > 0 ? Theme.warning : .primary,
                    systemImage: "exclamationmark.octagon")
            }
            Text(snapshot.costText)
                .font(.title3)
                .bold()
                .monospacedDigit()
                .contentTransition(.numericText())
                .foregroundStyle(Theme.color(for: dayPercent))
            if let dayPercent {
                MeterView(
                    label: "Day budget", percent: dayPercent, detail: snapshot.budget.paceNote)
            }
            if let weekly = snapshot.limits?.weekly {
                MeterView(
                    label: "Plan week", percent: weekly.usedPercent,
                    detail: snapshot.limits?.planType ?? "")
            }
            if let fiveHour = snapshot.limits?.fiveHour {
                MeterView(label: "Plan five hours", percent: fiveHour.usedPercent)
            }
            if snapshot.costIsEstimate {
                Text("The plan has no price for each call. The daemon computes these figures, so they are an estimate, not a charge.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            AgeNote(fetchedAt: fetchedAt, isStale: isStale)
        }
        .card()
    }

    private var dayPercent: Double? {
        let limit = snapshot.budget.dayLimitUsd
        guard limit > 0 else { return nil }
        return snapshot.budget.dayUsd / limit * 100
    }

    private var modeText: String { snapshot.mode.isEmpty ? "unknown" : snapshot.mode }

    private var modeColor: Color {
        switch snapshot.mode {
        case "fast": return Theme.normal
        case "economy": return Theme.warning
        default: return .secondary
        }
    }

    private var modeIcon: String {
        switch snapshot.mode {
        case "fast": return "hare.fill"
        case "economy": return "leaf.fill"
        case "paused": return "pause.fill"
        default: return "questionmark"
        }
    }
}
