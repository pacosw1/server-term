import SwiftUI

/// StatTile shows one headline number. An unknown value shows the dash.
struct StatTile: View {
    let label: String
    let value: String
    var tint: Color = Theme.text
    var systemImage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Label {
                Text(label)
                    .font(.footnote)
            } icon: {
                if let systemImage {
                    Image(systemName: systemImage).font(.footnote)
                }
            }
            .foregroundStyle(Theme.muted)
            Text(value)
                .font(.title3)
                .bold()
                .monospacedDigit()
                .contentTransition(.numericText())
                .foregroundStyle(tint)
                .minimumScaleFactor(0.7)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}
