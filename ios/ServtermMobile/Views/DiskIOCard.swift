import SwiftUI
import ServtermKit

/// DiskIOCard shows the read and the write rate of every block device. It
/// names the mount only when the device name matches exactly.
struct DiskIOCard: View {
    let entries: [DiskIOEntry]
    let disks: [DiskEntry]
    let ratesKnown: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Disk traffic").font(.headline)
            if !ratesKnown {
                Text("A rate needs two readings. The app shows the totals until the second one arrives.")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(DiskIOEntry.sortedByRate(entries)) { entry in
                VStack(alignment: .leading, spacing: 4) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(entry.device)
                            .font(.subheadline)
                            .bold()
                        Spacer(minLength: 8)
                        if let mount = DiskIOEntry.mount(forDevice: entry.device, in: disks) {
                            Text(mount)
                                .font(.caption)
                                .foregroundStyle(Theme.muted)
                        }
                    }
                    HStack(spacing: 12) {
                        Label(Format.rate(bytesPerSecond: entry.readRate, known: ratesKnown), systemImage: "arrow.down")
                        Label(Format.rate(bytesPerSecond: entry.writeRate, known: ratesKnown), systemImage: "arrow.up")
                    }
                    .font(.subheadline)
                    .monospacedDigit()
                    .contentTransition(.numericText())
                    .foregroundStyle(Theme.text)
                    Text("\(Format.bytes(unsigned: entry.readBytes)) read · \(Format.bytes(unsigned: entry.writeBytes)) written")
                        .font(.caption)
                        .monospacedDigit()
                        .foregroundStyle(Theme.muted)
                }
                .padding(.vertical, 8)
                .padding(.horizontal, 12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.raised)
                .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
            }
        }
        .card()
    }
}
