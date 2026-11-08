import AVFoundation
import SwiftUI

private struct ConnectionAttempt: Equatable {
    let id = UUID()
    let host: String
}

struct ConnectionView: View {
    var restoring = false
    var restorationError: String?
    var onAttempt: () -> Void = {}
    let onConnect: (String, [ClusterContext]) -> Void
    @Environment(\.scenePhase) private var scenePhase
    @State private var cameraPermission = AVCaptureDevice.authorizationStatus(for: .video)
    @State private var cameraError: String?
    @State private var connectionError: String?
    @State private var manualEntry = false
    @State private var address = ""
    @State private var attempt: ConnectionAttempt?
    @State private var scannerID = UUID()
    @State private var scanPaused = false
    @FocusState private var addressFocused: Bool

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                VStack(alignment: .leading, spacing: 10) {
                    Text("Connect your desktop").font(.largeTitle.bold())
                    Text("Open NaKube on your desktop and scan its connection QR code. Keep both devices connected to Tailscale.")
                        .foregroundStyle(.secondary)
                }
                if restoring {
                    ProgressView("Checking your previous desktop…")
                }
                scanner
                    .frame(maxWidth: .infinity)
                    .frame(height: 280)
                    .background(Color(uiColor: .secondarySystemBackground), in: RoundedRectangle(cornerRadius: 20))
                    .clipShape(RoundedRectangle(cornerRadius: 20))
                if let attempt {
                    HStack(spacing: 12) {
                        ProgressView()
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Connecting to desktop…").font(.headline)
                            Text(attempt.host).font(.subheadline).foregroundStyle(.secondary)
                        }
                    }
                    Button("Cancel Connection") { cancelAttempt() }
                }
                if let error = connectionError ?? restorationError {
                    ErrorMessageView(title: "Connection not completed", message: error)
                }
                if scanPaused && attempt == nil {
                    Button { resumeScanning() } label: {
                        Label("Scan Again", systemImage: "qrcode.viewfinder")
                            .frame(maxWidth: .infinity, minHeight: 44)
                    }
                    .buttonStyle(.borderedProminent)
                    .accessibilityIdentifier("connection.scanAgain")
                }
                if manualEntry {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Desktop address").font(.headline)
                        TextField("desktop.tailnet.ts.net", text: $address, axis: .vertical)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .keyboardType(.URL)
                            .textContentType(.URL)
                            .textFieldStyle(.roundedBorder)
                            .focused($addressFocused)
                            .accessibilityLabel("Desktop Tailscale address or NaKube link")
                            .disabled(attempt != nil)
                            .accessibilityIdentifier("connection.address")
                        Text("Enter the full .ts.net MagicDNS name shown on your desktop, or paste its NaKube link.")
                            .font(.footnote).foregroundStyle(.secondary)
                        Button { submit(address) } label: {
                            Text("Connect").frame(maxWidth: .infinity, minHeight: 44)
                        }
                        .buttonStyle(.borderedProminent)
                        .disabled(address.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || attempt != nil)
                        .accessibilityIdentifier("connection.connect")
                    }
                } else {
                    Button {
                        manualEntry = true
                        scanPaused = true
                        addressFocused = true
                    } label: {
                        Label("Enter Address Instead", systemImage: "keyboard")
                            .frame(maxWidth: .infinity, minHeight: 44)
                    }
                    .buttonStyle(.bordered)
                    .disabled(attempt != nil)
                    .accessibilityIdentifier("connection.manualEntry")
                }
            }
            .padding(24)
            .frame(maxWidth: 560)
            .frame(maxWidth: .infinity)
        }
        .navigationTitle("NaKube")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: attempt) {
            guard let current = attempt else { return }
            do {
                let contexts = try await DesktopAPI(host: current.host).contexts()
                try Task.checkCancellation()
                guard attempt == current else { return }
                onConnect(current.host, contexts)
            } catch {
                guard !Task.isCancelled, attempt == current else { return }
                connectionError = connectionMessage(error)
                attempt = nil
            }
        }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active {
                cameraPermission = AVCaptureDevice.authorizationStatus(for: .video)
                scannerID = UUID()
            }
        }
        .onDisappear { cancelAttempt() }
    }

    @ViewBuilder
    private var scanner: some View {
        if attempt != nil || scanPaused {
            ContentUnavailableView {
                Label(attempt == nil ? "Scanner Paused" : "QR Code Read", systemImage: "qrcode")
            } description: {
                Text(attempt == nil ? "Scan again or enter your desktop address below." : "Checking the desktop before saving this connection.")
            }
        } else if cameraPermission == .denied || cameraPermission == .restricted {
            cameraRecovery(
                title: "Camera Access Needed",
                message: "Allow camera access in Settings to scan, or enter the desktop address below.",
                settings: true
            )
        } else if let cameraError {
            cameraRecovery(title: "Camera Unavailable", message: cameraError, settings: false)
        } else {
            ZStack(alignment: .bottom) {
                QRScannerView(onCode: submit, onError: { message in
                    cameraPermission = AVCaptureDevice.authorizationStatus(for: .video)
                    cameraError = message
                })
                    .id(scannerID)
                    .accessibilityLabel("Camera scanning for the desktop QR code")
                Text("Point your camera at the desktop QR code")
                    .font(.footnote.weight(.medium))
                    .padding(12)
                    .frame(maxWidth: .infinity)
                    .background(.regularMaterial)
            }
        }
    }

    private func cameraRecovery(title: String, message: String, settings: Bool) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "camera").font(.largeTitle).foregroundStyle(.secondary)
            Text(title).font(.headline)
            Text(message).font(.subheadline).foregroundStyle(.secondary).multilineTextAlignment(.center)
            if settings && cameraPermission != .restricted {
                Button("Open Settings") {
                    guard let url = URL(string: UIApplication.openSettingsURLString) else { return }
                    UIApplication.shared.open(url)
                }
                .buttonStyle(.bordered)
            } else if !settings {
                Button("Retry Camera") {
                    cameraError = nil
                    cameraPermission = AVCaptureDevice.authorizationStatus(for: .video)
                    scannerID = UUID()
                }
                .buttonStyle(.bordered)
            }
        }
        .padding()
    }

    private func submit(_ value: String) {
        guard attempt == nil else { return }
        onAttempt()
        addressFocused = false
        scanPaused = true
        connectionError = nil
        guard let host = tailscaleHost(from: value) else {
            connectionError = RemoteError.invalidHost.localizedDescription
            return
        }
        address = host
        attempt = ConnectionAttempt(host: host)
    }

    private func cancelAttempt() {
        attempt = nil
        scanPaused = true
    }

    private func resumeScanning() {
        connectionError = nil
        cameraError = nil
        cameraPermission = AVCaptureDevice.authorizationStatus(for: .video)
        manualEntry = false
        addressFocused = false
        scannerID = UUID()
        scanPaused = false
    }
}
