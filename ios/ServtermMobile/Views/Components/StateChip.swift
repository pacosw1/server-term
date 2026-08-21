import SwiftUI

/// StateChip shows one state in one word. It carries a symbol as well as a
/// colour, so the state is readable without colour, and a hard 2 pixel
/// border in its own colour, so it is readable at a glance.
struct StateChip: View {
    let text: String
    let color: Color
    var systemImage: String?

    var body: some View {
        Label {
            Text(text.uppercased())
                .font(.caption)
                .bold()
                .kerning(0.6)
        } icon: {
            if let systemImage {
                Image(systemName: systemImage)
                    .font(.caption)
            }
        }
        .labelStyle(.titleAndIcon)
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .foregroundStyle(color)
        .background(Theme.base)
        .overlay {
            RoundedRectangle(cornerRadius: Theme.cardRadius)
                .strokeBorder(color, lineWidth: Theme.borderWidth)
        }
        .clipShape(.rect(cornerRadius: Theme.cardRadius))
        .accessibilityLabel(text)
    }
}
