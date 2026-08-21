import SwiftUI
import ServtermKit

struct ServerAcceleratorCard: View {
    let accelerators: [AcceleratorEntry]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Accelerators").font(.headline)
            ForEach(accelerators) { item in
                MeterView(
                    label: "\(item.kind) \(item.name)", percent: item.utilizationPercent,
                    detail: item.utilizationKnown ? "" : "the driver reports no reading")
            }
        }
        .card()
    }
}
