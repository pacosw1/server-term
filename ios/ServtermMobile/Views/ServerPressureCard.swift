import SwiftUI
import ServtermKit

/// ServerPressureCard shows the Linux pressure readings. A host that does
/// not report them shows no card at all, instead of three zeros.
struct ServerPressureCard: View {
    let sample: Sample

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Pressure").font(.headline)
            Text("The share of the time that work waited for a resource.")
                .font(.footnote)
                .foregroundStyle(.secondary)
            MeterView(label: "CPU", percent: sample.pressureCPU)
            MeterView(label: "Memory", percent: sample.pressureMemory)
            MeterView(label: "Input and output", percent: sample.pressureIO)
        }
        .card()
    }
}
