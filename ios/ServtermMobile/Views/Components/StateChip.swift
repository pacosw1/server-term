import SwiftUI

/// StateChip shows one state in one word. It carries a shape as well as a
/// colour, so the state is readable without colour.
struct StateChip: View {
    let text: String
    let color: Color
    var systemImage: String?

    var body: some View {
        Label {
            Text(text)
                .font(.caption)
                .bold()
        } icon: {
            if let systemImage {
                Image(systemName: systemImage)
                    .font(.caption)
            }
        }
        .labelStyle(.titleAndIcon)
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(color.opacity(0.16), in: .capsule)
        .foregroundStyle(color)
        .accessibilityLabel(text)
    }
}
