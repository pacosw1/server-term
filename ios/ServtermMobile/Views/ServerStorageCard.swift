import SwiftUI
import ServtermKit

struct ServerStorageCard: View {
    let disks: [DiskEntry]

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Storage").font(.headline)
            if disks.isEmpty {
                Text("The agent reports no disk.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            ForEach(disks) { disk in
                MeterView(
                    label: disk.mount, percent: disk.usedPercent,
                    detail: "\(Format.bytes(unsigned: disk.used)) of \(Format.bytes(unsigned: disk.total)) · \(disk.fsType)")
            }
        }
        .card()
    }
}
