import SwiftUI

/// ErrorBanner shows the last failure. The reading above it keeps its own
/// age mark, so no screen shows an old value as a fresh one.
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
        .foregroundStyle(.white)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.critical.opacity(0.92), in: .rect(cornerRadius: 12))
    }
}
