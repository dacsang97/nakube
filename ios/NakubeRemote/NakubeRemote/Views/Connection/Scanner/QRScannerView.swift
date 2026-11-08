import SwiftUI

struct QRScannerView: UIViewControllerRepresentable {
    let onCode: (String) -> Void
    let onError: (String) -> Void

    func makeUIViewController(context: Context) -> QRScannerViewController {
        QRScannerViewController(onCode: onCode, onError: onError)
    }

    func updateUIViewController(_ controller: QRScannerViewController, context: Context) {}

    static func dismantleUIViewController(_ controller: QRScannerViewController, coordinator: ()) {
        controller.stop()
    }
}
