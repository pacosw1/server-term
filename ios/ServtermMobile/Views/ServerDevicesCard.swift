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
                LabeledContent {
                    Text(Format.bytes(unsigned: device.size))
                        .monospacedDigit()
                } label: {
                    Label(device.name, systemImage: device.kind == "ssd" ? "internaldrive" : "opticaldiscdrive")
                }
            }
        }
        .font(.subheadline)
        .card()
    }
}
