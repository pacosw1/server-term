import SwiftUI
import ServtermKit

/// InterfacesCard lists the network interfaces, the busiest first. A host
/// that runs containers carries many quiet pairs, so the quiet ones sit
/// behind a disclosure.
struct InterfacesCard: View {
    let interfaces: [InterfaceEntry]
    let ratesKnown: Bool
    @State private var showsAll = false

    private var split: (busy: [InterfaceEntry], rest: [InterfaceEntry]) {
        InterfaceEntry.split(interfaces, limit: 6)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                Text("Interfaces").font(.headline)
                Spacer(minLength: 8)
                Text("\(interfaces.count)")
                    .font(.subheadline)
                    .monospacedDigit()
                    .foregroundStyle(Theme.muted)
            }
            if interfaces.isEmpty {
                Text("The host reports no interface.")
                    .font(.footnote)
                    .foregroundStyle(Theme.muted)
            }
            if !ratesKnown && !interfaces.isEmpty {
                Text("A rate needs two readings. The app shows the totals until the second one arrives.")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            ForEach(split.busy) { item in
                InterfaceRow(item: item, ratesKnown: ratesKnown)
            }
            if !split.rest.isEmpty {
                Button {
                    withAnimation(.easeInOut(duration: 0.2)) { showsAll.toggle() }
                } label: {
                    Label(
                        showsAll
                            ? "Hide the quiet interfaces"
                            : "Show \(split.rest.count) quiet interfaces",
                        systemImage: showsAll ? "chevron.up" : "chevron.down")
                        .font(.subheadline)
                        .foregroundStyle(Theme.accent)
                        .frame(minHeight: Theme.minimumTapTarget, alignment: .leading)
                }
                .buttonStyle(.plain)
                if showsAll {
                    ForEach(split.rest) { item in
                        InterfaceRow(item: item, ratesKnown: ratesKnown)
                    }
                }
            }
        }
        .card()
    }
}

/// InterfaceRow shows the rate as the headline and the totals as the
/// detail. An error or a drop carries a warning mark; a clean interface
/// stays quiet.
struct InterfaceRow: View {
    let item: InterfaceEntry
    let ratesKnown: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .firstTextBaseline) {
                Text(item.name)
                    .font(.subheadline)
                    .bold()
                    .lineLimit(1)
                Spacer(minLength: 8)
                if item.hasFaults {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .font(.caption)
                        .foregroundStyle(Theme.warning)
                        .accessibilityLabel("this interface counted errors or drops")
                }
            }
            HStack(spacing: 12) {
                Label(Format.rate(bytesPerSecond: item.rxRate, known: ratesKnown), systemImage: "arrow.down")
                Label(Format.rate(bytesPerSecond: item.txRate, known: ratesKnown), systemImage: "arrow.up")
            }
            .font(.subheadline)
            .monospacedDigit()
            .contentTransition(.numericText())
            .foregroundStyle(Theme.text)
            Text("\(Format.bytes(unsigned: item.rx)) in · \(Format.bytes(unsigned: item.tx)) out")
                .font(.caption)
                .monospacedDigit()
                .foregroundStyle(Theme.muted)
            if item.hasFaults {
                Text("errors \(item.rxErrors) in / \(item.txErrors) out · drops \(item.rxDrops) in / \(item.txDrops) out")
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(Theme.warning)
            }
        }
        .padding(.vertical, 8)
        .padding(.horizontal, 12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.raised)
        .overlay { Rectangle().strokeBorder(Theme.border, lineWidth: 1) }
    }
}
