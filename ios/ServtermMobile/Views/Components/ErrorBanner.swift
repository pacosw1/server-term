import SwiftUI

/// ErrorBanner shows the last failure. It is an outlined block, not a
/// filled one: the text then holds the AAA contrast that a solid red fill
/// cannot give, and the heavy red border keeps it loud.
struct ErrorBanner: View {
    let message: String

    var body: some View {
        Label {
            Text(message)
                .font(.footnote)
                .fixedSize(horizontal: false, vertical: true)
        } icon: {
            Image(systemName: "exclamationmark.triangle.fill")
        }
        .foregroundStyle(Theme.critical)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .overlay {
            RoundedRectangle(cornerRadius: Theme.cardRadius)
                .strokeBorder(Theme.dangerFill, lineWidth: Theme.borderWidth)
        }
        .clipShape(.rect(cornerRadius: Theme.cardRadius))
    }
}
