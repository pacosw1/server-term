import SwiftUI

/// InfoRow is one label and one value. It replaces LabeledContent, whose
/// system value colour is dimmer than the AAA target of this theme.
struct InfoRow: View {
    let label: String
    let value: String

    init(_ label: String, value: String) {
        self.label = label
        self.value = value
    }

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            Text(label)
                .foregroundStyle(Theme.muted)
            Spacer(minLength: 12)
            Text(value)
                .foregroundStyle(Theme.text)
                .monospacedDigit()
                .multilineTextAlignment(.trailing)
        }
        .accessibilityElement(children: .combine)
    }
}
