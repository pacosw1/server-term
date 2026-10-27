import SwiftUI
import UIKit
import VisionKit

/// PairingView scans the QR code that `servterm pair` shows on the Mac, and
/// adds every server in it. A paste button covers a phone without a camera
/// and the simulator.
struct PairingView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var result: (ok: Bool, message: String)?
    /// lastRejected stops the scanner from reporting the same wrong code
    /// many times a second.
    @State private var lastRejected: String?

    private var canScan: Bool {
        DataScannerViewController.isSupported && DataScannerViewController.isAvailable
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                ZStack {
                    Theme.base
                    if canScan {
                        QRScanner { text in handle(text) }
                            .ignoresSafeArea(edges: .bottom)
                    } else {
                        ContentUnavailableView {
                            Label("Camera unavailable", systemImage: "camera")
                        } description: {
                            Text("Run `servterm pair --file setup.json` on your Mac, copy the file's text, and paste it here.")
                        }
                    }
                    if let result, result.ok {
                        Label(result.message, systemImage: "checkmark.circle.fill")
                            .font(.title3.bold())
                            .foregroundStyle(Theme.base)
                            .padding()
                            .background(Theme.normal)
                            .clipShape(.rect(cornerRadius: Theme.cardRadius))
                    }
                }
                footer
            }
            .background(Theme.base)
            .navigationTitle("Scan pairing code")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }

    private var footer: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let result, !result.ok {
                ErrorBanner(message: result.message)
            }
            Text("On your Mac, run **servterm pair** and point the camera at the code.")
                .font(.subheadline)
                .foregroundStyle(Theme.muted)
            Button {
                if let text = UIPasteboard.general.string { handle(text) }
            } label: {
                Label("Paste code instead", systemImage: "doc.on.clipboard")
                    .font(.subheadline)
                    .frame(maxWidth: .infinity, minHeight: Theme.minimumTapTarget)
            }
            .buttonStyle(.bordered)
        }
        .padding()
        .background(Theme.surface)
    }

    private func handle(_ text: String) {
        guard result?.ok != true, text != lastRejected else { return }
        let outcome = model.importPairing(text)
        result = outcome
        if outcome.ok {
            UINotificationFeedbackGenerator().notificationOccurred(.success)
            Task {
                try? await Task.sleep(for: .seconds(1.2))
                dismiss()
            }
        } else {
            lastRejected = text
            UINotificationFeedbackGenerator().notificationOccurred(.error)
        }
    }
}

/// QRScanner is the live camera view of VisionKit, set to QR codes only.
private struct QRScanner: UIViewControllerRepresentable {
    let onCode: (String) -> Void

    func makeUIViewController(context: Context) -> DataScannerViewController {
        let scanner = DataScannerViewController(
            recognizedDataTypes: [.barcode(symbologies: [.qr])],
            qualityLevel: .balanced,
            isHighlightingEnabled: true)
        scanner.delegate = context.coordinator
        try? scanner.startScanning()
        return scanner
    }

    func updateUIViewController(_ scanner: DataScannerViewController, context: Context) {
        context.coordinator.onCode = onCode
    }

    static func dismantleUIViewController(_ scanner: DataScannerViewController, coordinator: Coordinator) {
        scanner.stopScanning()
    }

    func makeCoordinator() -> Coordinator { Coordinator(onCode: onCode) }

    @MainActor
    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        var onCode: (String) -> Void

        init(onCode: @escaping (String) -> Void) {
            self.onCode = onCode
        }

        func dataScanner(
            _ dataScanner: DataScannerViewController, didAdd addedItems: [RecognizedItem],
            allItems: [RecognizedItem]
        ) {
            for item in addedItems {
                if case .barcode(let code) = item, let text = code.payloadStringValue {
                    onCode(text)
                    return
                }
            }
        }
    }
}
