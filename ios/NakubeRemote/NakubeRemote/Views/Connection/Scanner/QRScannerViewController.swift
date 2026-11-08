import AVFoundation
import UIKit

@MainActor
final class QRScannerViewController: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
    private let capture = CameraCapture()
    private let onCode: (String) -> Void
    private let onError: (String) -> Void
    private var preview: AVCaptureVideoPreviewLayer?
    private var active = false
    private var delivered = false

    init(onCode: @escaping (String) -> Void, onError: @escaping (String) -> Void) {
        self.onCode = onCode
        self.onError = onError
        super.init(nibName: nil, bundle: nil)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is unavailable") }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black
        let preview = AVCaptureVideoPreviewLayer(session: capture.session)
        preview.videoGravity = .resizeAspectFill
        view.layer.addSublayer(preview)
        self.preview = preview
        NotificationCenter.default.addObserver(self, selector: #selector(captureFailed), name: .AVCaptureSessionRuntimeError, object: capture.session)
        NotificationCenter.default.addObserver(self, selector: #selector(captureInterrupted), name: .AVCaptureSessionWasInterrupted, object: capture.session)
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        active = true
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            start()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
                Task { @MainActor [weak self] in
                    guard let self, self.active else { return }
                    if granted { self.start() }
                    else { self.onError("Camera access was not allowed. Open Settings to allow access, or enter the desktop address instead.") }
                }
            }
        default:
            onError("Allow camera access in Settings, or enter the desktop address instead.")
        }
    }

    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        stop()
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        preview?.frame = view.bounds
        if let connection = preview?.connection, connection.isVideoRotationAngleSupported(90) {
            let angle: CGFloat
            switch view.window?.windowScene?.interfaceOrientation {
            case .landscapeLeft: angle = 0
            case .landscapeRight: angle = 180
            case .portraitUpsideDown: angle = 270
            default: angle = 90
            }
            if connection.isVideoRotationAngleSupported(angle) { connection.videoRotationAngle = angle }
        }
    }

    func stop() {
        active = false
        capture.stop()
    }

    private func start() {
        guard active, !delivered else { return }
        capture.start(delegate: self) { [weak self] message in
            guard let self, self.active else { return }
            self.onError(message)
        }
    }

    @objc nonisolated private func captureFailed() {
        Task { @MainActor [weak self] in
            guard let self, self.active else { return }
            self.onError("The camera stopped unexpectedly. Retry the camera or enter the desktop address instead.")
        }
    }

    @objc nonisolated private func captureInterrupted() {
        Task { @MainActor [weak self] in
            guard let self, self.active else { return }
            self.onError("The camera was interrupted. Close other camera apps and retry, or enter the desktop address instead.")
        }
    }

    nonisolated func metadataOutput(
        _ output: AVCaptureMetadataOutput,
        didOutput metadataObjects: [AVMetadataObject],
        from connection: AVCaptureConnection
    ) {
        guard let code = metadataObjects.lazy.compactMap({ ($0 as? AVMetadataMachineReadableCodeObject)?.stringValue }).first else { return }
        Task { @MainActor [weak self] in
            guard let self, self.active, !self.delivered else { return }
            self.delivered = true
            self.capture.stop()
            self.onCode(code)
        }
    }

    deinit { NotificationCenter.default.removeObserver(self) }
}
