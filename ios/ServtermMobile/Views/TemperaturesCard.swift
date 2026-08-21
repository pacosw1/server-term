import SwiftUI
import ServtermKit

/// TemperaturesCard lists the sensors, the hottest first. The agent sends
/// no limit with a reading, so the card shows the number and grades none
/// of them.
struct TemperaturesCard: View {
    let temperatures: [Temperature]

    private let columns = [GridItem(.adaptive(minimum: 132), spacing: 8)]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                Text("Temperatures").font(.headline)
                Spacer(minLength: 8)
                Text("\(temperatures.count) sensors")
                    .font(.subheadline)
                    .foregroundStyle(Theme.muted)
            }
            LazyVGrid(columns: columns, alignment: .leading, spacing: 8) {
                ForEach(Temperature.sortedByHeat(temperatures)) { sensor in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(sensor.label)
                            .font(.caption)
                            .foregroundStyle(Theme.muted)
                            .lineLimit(1)
                        Text(Format.celsius(sensor.celsius))
                            .font(.subheadline)
                            .bold()
                            .monospacedDigit()
                            .contentTransition(.numericText())
                            .foregroundStyle(Theme.text)
                    }
                    .padding(8)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Theme.raised)
                    .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
                    .accessibilityElement(children: .combine)
                }
            }
        }
        .card()
    }
}
