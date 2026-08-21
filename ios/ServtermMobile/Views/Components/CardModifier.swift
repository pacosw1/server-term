import SwiftUI

/// CardModifier gives every block the same shape, so the screens read as
/// one design in the light appearance and in the dark appearance.
struct CardModifier: ViewModifier {
    @Environment(\.colorScheme) private var colorScheme

    func body(content: Content) -> some View {
        content
            .padding(Theme.cardPadding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(fill, in: .rect(cornerRadius: Theme.cardRadius))
            .shadow(color: .black.opacity(colorScheme == .dark ? 0 : 0.07), radius: 10, y: 3)
            .overlay {
                RoundedRectangle(cornerRadius: Theme.cardRadius)
                    .strokeBorder(.quaternary, lineWidth: 1)
            }
    }
}

extension CardModifier {
    /// fill keeps the card above the page: a white card in the light
    /// appearance, and a lighter grey card in the dark appearance.
    private var fill: AnyShapeStyle {
        colorScheme == .dark ? AnyShapeStyle(.background.secondary) : AnyShapeStyle(.background)
    }
}

extension View {
    func card() -> some View {
        modifier(CardModifier())
    }
}

/// PageBackground puts a soft accent wash behind every screen, so the dark
/// appearance looks chosen instead of plain grey.
struct PageBackground: View {
    var body: some View {
        LinearGradient(
            colors: [Theme.accent.opacity(0.10), .clear],
            startPoint: .top, endPoint: .center)
            .ignoresSafeArea()
            .background(Color(.systemGroupedBackground).ignoresSafeArea())
    }
}
