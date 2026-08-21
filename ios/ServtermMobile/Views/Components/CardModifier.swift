import SwiftUI

/// CardModifier gives every block the same shape: a flat card on the page,
/// sharp 2 point corners, and a heavy 2 pixel border. There is no shadow
/// and no glow, because the theme is flat.
struct CardModifier: ViewModifier {
    func body(content: Content) -> some View {
        content
            .padding(Theme.cardPadding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.surface)
            .overlay {
                RoundedRectangle(cornerRadius: Theme.cardRadius)
                    .strokeBorder(Theme.border, lineWidth: Theme.borderWidth)
            }
            .clipShape(.rect(cornerRadius: Theme.cardRadius))
    }
}

extension View {
    func card() -> some View {
        modifier(CardModifier())
    }
}

/// PageBackground is one flat colour. The plain backdrop carries no wash
/// and no gradient.
struct PageBackground: View {
    var body: some View {
        Theme.base.ignoresSafeArea()
    }
}
