import AVFoundation

// AVCaptureSession is a queue-confined resource. Configuration and running-state
// mutations are serialized here, never on the UI thread. The preview layer is
// attached once on the main thread before any capture work is enqueued.
final class CameraCapture: @unchecked Sendable {
    let session = AVCaptureSession()
    private let queue = DispatchQueue(label: "space.gnas.nakube.camera")
    private var configured = false

    func start(delegate: QRScannerViewController, onError: @escaping @MainActor @Sendable (String) -> Void) {
        queue.async { [self] in
            if !configured {
                guard let camera = AVCaptureDevice.default(for: .video) else {
                    Task { @MainActor in onError("No camera is available. Enter the desktop address instead.") }
                    return
                }
                do {
                    let input = try AVCaptureDeviceInput(device: camera)
                    let output = AVCaptureMetadataOutput()
                    session.beginConfiguration()
                    guard session.canAddInput(input), session.canAddOutput(output) else {
                        session.commitConfiguration()
                        Task { @MainActor in onError("This camera cannot scan QR codes. Enter the desktop address instead.") }
                        return
                    }
                    session.addInput(input)
                    session.addOutput(output)
                    output.setMetadataObjectsDelegate(delegate, queue: .main)
                    guard output.availableMetadataObjectTypes.contains(.qr) else {
                        session.removeOutput(output)
                        session.removeInput(input)
                        session.commitConfiguration()
                        Task { @MainActor in onError("QR scanning is unavailable. Enter the desktop address instead.") }
                        return
                    }
                    output.metadataObjectTypes = [.qr]
                    session.commitConfiguration()
                    configured = true
                } catch {
                    Task { @MainActor in onError("The camera could not be opened. Try again or enter the desktop address instead.") }
                    return
                }
            }
            if !session.isRunning { session.startRunning() }
        }
    }

    func stop() {
        queue.async { [self] in
            if session.isRunning { session.stopRunning() }
        }
    }
}
