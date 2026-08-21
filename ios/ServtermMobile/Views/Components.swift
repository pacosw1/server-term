import SwiftUI
import ServtermKit

/// pollEvery repeats an action while the screen is visible. SwiftUI cancels
/// the task when the screen goes away, so the app stops reading then.
extension View {
    func pollEvery(seconds: Double = 3, action: @escaping @MainActor () async -> Void) -> some View {
        task {
            while !Task.isCancelled {
                await action()
                try? await Task.sleep(for: .seconds(seconds))
            }
        }
    }
}

/// ErrorBanner shows the last failure. It never replaces a reading with a
/// zero: the reading above it keeps its own age mark.
struct ErrorBanner: View {
    let message: String

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
            Text(message)
                .font(.footnote)
                .fixedSize(horizontal: false, vertical: true)
        }
        .foregroundStyle(.white)
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.red.opacity(0.85), in: RoundedRectangle(cornerRadius: 8))
        .listRowInsets(EdgeInsets(top: 4, leading: 16, bottom: 4, trailing: 16))
    }
}

/// AgeNote says how old the reading is, so no screen shows an old value as
/// a fresh one.
struct AgeNote: View {
    let fetchedAt: Date?
    let isStale: Bool

    var body: some View {
        Group {
            if let fetchedAt {
                Text("read \(Format.relativeAge(fetchedAt))")
            } else {
                Text("no reading yet")
            }
        }
        .font(.caption2)
        .foregroundStyle(isStale ? Color.orange : Color.secondary)
    }
}

/// MetricRow shows one label and one value.
struct MetricRow: View {
    let label: String
    let value: String

    var body: some View {
        HStack {
            Text(label)
                .foregroundStyle(.secondary)
            Spacer()
            Text(value)
                .monospacedDigit()
        }
    }
}

/// PercentBar draws a bar only when the app knows the value. An unknown
/// value shows the dash, because an empty bar reads as "plenty left".
struct PercentBar: View {
    let label: String
    let percent: Double?
    var detail: String = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(label).foregroundStyle(.secondary)
                Spacer()
                Text(Format.optionalPercent(percent)).monospacedDigit()
            }
            if let percent {
                ProgressView(value: min(max(percent, 0), 100), total: 100)
                    .tint(color(for: percent))
            }
            if !detail.isEmpty {
                Text(detail).font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    private func color(for value: Double) -> Color {
        if value >= 90 { return .red }
        if value >= 70 { return .orange }
        return .accentColor
    }
}

/// StateBadge shows the state of one service in one word.
struct StateBadge: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.caption2.weight(.semibold))
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.18), in: Capsule())
            .foregroundStyle(color)
    }
}

/// EmptyHint tells the user what to do next.
struct EmptyHint: View {
    let title: String
    let message: String

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: "list.bullet.rectangle")
        } description: {
            Text(message)
        }
    }
}
