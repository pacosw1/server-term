import SwiftUI
import ServtermKit

/// ServerProcessCard lists the processes. The reader chooses the order.
struct ServerProcessCard: View {
    let sample: Sample
    @State private var order: ProcessSort = .cpu

    private var processes: [ProcessEntry] { sample.processes(sortedBy: order) }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Processes").font(.headline)
                Spacer(minLength: 8)
            }
            Picker("Order", selection: $order) {
                ForEach(ProcessSort.allCases) { option in
                    Text(option.label).tag(option)
                }
            }
            .pickerStyle(.segmented)
            if processes.isEmpty {
                Text("The agent reports no process.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(processes.prefix(15)) { process in
                VStack(alignment: .leading, spacing: 3) {
                    HStack {
                        Text(process.command)
                            .font(.subheadline)
                            .lineLimit(1)
                        Spacer(minLength: 8)
                        Text(order == .cpu
                            ? Format.percent(process.cpu)
                            : Format.bytes(unsigned: process.rss))
                            .font(.subheadline)
                            .monospacedDigit()
                            .foregroundStyle(Theme.accent)
                    }
                    Text("pid \(process.pid) · \(process.user) · \(Format.percent(process.cpu)) cpu · \(Format.bytes(unsigned: process.rss))")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                }
                .padding(.vertical, 2)
            }
        }
        .card()
    }
}
