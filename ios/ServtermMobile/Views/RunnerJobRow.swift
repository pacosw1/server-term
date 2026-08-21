import SwiftUI
import ServtermKit

/// RunnerJobRow is one CI job that runs now.
struct RunnerJobRow: View {
    let job: RunnerJob
    let now: Date
    /// cpu is nil until the app holds two readings, so the row shows the
    /// dash instead of a false zero.
    let cpu: Double?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                Text(job.repository.isEmpty ? "unknown repository" : job.repository)
                    .font(.subheadline)
                    .bold()
                    .lineLimit(1)
                Spacer(minLength: 8)
                Text(Format.duration(seconds: job.elapsedSeconds(now: now)))
                    .font(.subheadline)
                    .monospacedDigit()
                    .contentTransition(.numericText())
                    .foregroundStyle(Theme.muted)
            }
            Text(job.title)
                .font(.subheadline)
                .foregroundStyle(Theme.muted)
                .lineLimit(2)
            HStack(spacing: 10) {
                // A job CPU above 100 percent only means more than one
                // core, so the row does not grade it as a warning.
                Label(Format.optionalPercent(cpu), systemImage: "cpu")
                    .foregroundStyle(cpu == nil ? Theme.muted : Theme.accent)
                Label(Format.bytes(unsigned: job.rss), systemImage: "memorychip")
                Label("\(job.processes)", systemImage: "square.stack.3d.up")
                if !job.runNumber.isEmpty {
                    Text("run \(job.runNumber)")
                }
            }
            .font(.caption)
            .monospacedDigit()
            .foregroundStyle(Theme.muted)
            if let url = job.runURL {
                Link("Open the run page", destination: url)
                    .font(.caption)
            }
        }
        .padding(.vertical, 8)
        .padding(.horizontal, 12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.raised)
        .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: Theme.borderWidth) }
    }
}
