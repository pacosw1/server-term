import SwiftUI
import ServtermKit

/// ServerDevicesCard lists the disks that the machine carries. The agent
/// reports the name, the kind and the size only.
struct ServerDevicesCard: View {
    let devices: [BlockDevice]

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Devices").font(.headline)
            ForEach(devices) { device in
                HStack {
                    Label(device.name, systemImage: device.kind == "ssd" ? "internaldrive" : "opticaldiscdrive")
                        .foregroundStyle(Theme.muted)
                    Spacer(minLength: 12)
                    Text(Format.bytes(unsigned: device.size))
                        .monospacedDigit()
                        .foregroundStyle(Theme.text)
                }
            }
        }
        .font(.subheadline)
        .card()
    }
}
