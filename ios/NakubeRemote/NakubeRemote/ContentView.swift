import AVFoundation
import SwiftUI

struct PodRow: Decodable, Identifiable {
    var id: String { "\(namespace)/\(name)" }
    let namespace: String
    let name: String
    let phase: String
    let owned: Bool
}

struct OverviewBody: Decodable {
    let context: String
    let nodes: String
    let pods: String
    let warnings: String
    let cpu: String
    let memory: String
}

struct ArgoRow: Decodable, Identifiable {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let sync: String
    let health: String
}

struct ContentView: View {
    @AppStorage("desktopHost") private var host = ""
    @State private var overview: OverviewBody?
    @State private var pods: [PodRow] = []
    @State private var apps: [ArgoRow] = []
    @State private var errorText = ""
    @State private var loading = false
    @State private var pending: PendingAction?
    @State private var showingScanner = false

    var body: some View {
        NavigationStack {
            List {
                Section("Desktop") {
                    TextField("Tailscale address", text: $host)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button {
                        showingScanner = true
                    } label: {
                        Label("Scan QR Code", systemImage: "qrcode.viewfinder")
                    }
                    Button(loading ? "Refreshing…" : "Refresh") { Task { await refresh() } }
                        .disabled(loading)
                }
                if !errorText.isEmpty {
                    Section { Text(errorText).foregroundStyle(.red) }
                }
                if loading && overview == nil && errorText.isEmpty {
                    loadingSections
                } else if let overview {
                    Section("Overview") {
                        LabeledContent("Context", value: overview.context)
                        LabeledContent("Nodes", value: overview.nodes)
                        LabeledContent("Pods", value: overview.pods)
                        LabeledContent("Warnings", value: overview.warnings)
                        LabeledContent("CPU", value: overview.cpu)
                        LabeledContent("Memory", value: overview.memory)
                    }
                }
                if !(loading && overview == nil && errorText.isEmpty) {
                    Section("Pods") {
                        ForEach(pods) { pod in
                            VStack(alignment: .leading) {
                                Text(pod.name)
                                Text("\(pod.namespace) · \(pod.phase)").font(.caption).foregroundStyle(.secondary)
                            }
                            .swipeActions {
                                Button("Delete", role: .destructive) {
                                    pending = PendingAction(pod: pod, restart: false)
                                }
                                Button("Restart") {
                                    pending = PendingAction(pod: pod, restart: true)
                                }
                            }
                        }
                    }
                    Section("Argo") {
                        ForEach(apps) { app in
                            VStack(alignment: .leading) {
                                Text(app.name)
                                Text("\(app.namespace) · \(app.sync) · \(app.health)").font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            .navigationTitle("NaKube")
            .confirmationDialog(pending?.title ?? "", isPresented: pendingPresented) {
                Button(pending?.verb ?? "Confirm", role: .destructive) {
                    Task { await confirmPending() }
                }
                Button("Cancel", role: .cancel) { pending = nil }
            } message: {
                Text(pending?.message ?? "")
            }
        }
        .sheet(isPresented: $showingScanner) {
            NavigationStack {
                QRScannerView(
                    onCode: connect,
                    onError: { message in
                        showingScanner = false
                        errorText = message
                    }
                )
                .ignoresSafeArea()
                .navigationTitle("Scan Na Kube")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { showingScanner = false }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var loadingSections: some View {
        Group {
            Section("Overview") {
                ForEach(["Context", "Nodes", "Pods", "Warnings", "CPU", "Memory"], id: \.self) { label in
                    LabeledContent(label, value: "000000")
                }
            }
            Section("Pods") {
                ForEach(0..<4, id: \.self) { _ in
                    VStack(alignment: .leading) {
                        Text("pod-name")
                        Text("namespace · Running").font(.caption)
                    }
                }
            }
            Section("Argo") {
                ForEach(0..<3, id: \.self) { _ in
                    VStack(alignment: .leading) {
                        Text("application")
                        Text("namespace · Synced · Healthy").font(.caption)
                    }
                }
            }
        }
        .redacted(reason: .placeholder)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Loading cluster")
    }

    private var pendingPresented: Binding<Bool> {
        Binding(get: { pending != nil }, set: { if !$0 { pending = nil } })
    }

    private func connect(code: String) {
        showingScanner = false
        guard let scannedHost = tailscaleHost(from: code) else {
            errorText = "That QR code is not a Na Kube Tailscale address."
            return
        }
        host = scannedHost
        Task { await refresh() }
    }

    private func refresh() async {
        loading = true
        defer { loading = false }
        errorText = ""
        guard let base = baseURL else {
            errorText = "Enter the desktop MagicDNS name."
            return
        }
        do {
            overview = try await get(base.appending(path: "v1/overview"))
            pods = try await get(base.appending(path: "v1/pods"))
            apps = try await get(base.appending(path: "v1/argo/applications"))
        } catch {
            errorText = error.localizedDescription
        }
    }

    private func confirmPending() async {
        guard let action = pending, let base = baseURL else { return }
        pending = nil
        let verb = action.restart ? "restart" : "delete"
        if action.restart && !action.pod.owned {
            errorText = "This pod has no controller. Use delete."
            return
        }
        var request = URLRequest(url: base.appending(path: "v1/pods/\(action.pod.namespace)/\(action.pod.name)/\(verb)"))
        request.httpMethod = "POST"
        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            if let http = response as? HTTPURLResponse, http.statusCode >= 300 {
                errorText = "The desktop refused \(verb)."
            }
            await refresh()
        } catch {
            errorText = error.localizedDescription
        }
    }

    private var baseURL: URL? {
        let trimmed = host.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return nil }
        return URL(string: "http://\(trimmed):47623")
    }

    private func get<T: Decodable>(_ url: URL) async throws -> T {
        let (data, response) = try await URLSession.shared.data(from: url)
        if let http = response as? HTTPURLResponse, http.statusCode >= 300 {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode(T.self, from: data)
    }
}

private func tailscaleHost(from code: String) -> String? {
    let trimmed = code.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let components = URLComponents(string: trimmed) else { return nil }
    let candidate: String?
    switch components.scheme?.lowercased() {
    case "nakube":
        candidate = components.queryItems?.first(where: { $0.name == "host" })?.value
    case "http", "https":
        candidate = components.host
    default:
        candidate = trimmed
    }
    guard let host = candidate?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased(),
          isTailscaleHost(host) else {
        return nil
    }
    return host
}

private func isTailscaleHost(_ host: String) -> Bool {
    if host.hasSuffix(".ts.net") {
        return host.count > ".ts.net".count
    }
    let octets = host.split(separator: ".", omittingEmptySubsequences: false).compactMap { Int($0) }
    guard octets.count == 4, octets.allSatisfy({ 0...255 ~= $0 }) else { return false }
    return octets[0] == 100 && (64...127).contains(octets[1])
}

private struct QRScannerView: UIViewControllerRepresentable {
    let onCode: (String) -> Void
    let onError: (String) -> Void

    func makeUIViewController(context: Context) -> QRScannerViewController {
        QRScannerViewController(onCode: onCode, onError: onError)
    }

    func updateUIViewController(_ controller: QRScannerViewController, context: Context) {}
}

private final class QRScannerViewController: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
    private let session = AVCaptureSession()
    private let onCode: (String) -> Void
    private let onError: (String) -> Void
    private var configured = false
    private var delivered = false

    init(onCode: @escaping (String) -> Void, onError: @escaping (String) -> Void) {
        self.onCode = onCode
        self.onError = onError
        super.init(nibName: nil, bundle: nil)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) {
        fatalError("init(coder:) is unavailable")
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            configureAndStart()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
                DispatchQueue.main.async {
                    if granted {
                        self?.configureAndStart()
                    } else {
                        self?.onError("Camera access is required to scan the desktop QR code.")
                    }
                }
            }
        default:
            onError("Allow camera access in Settings to scan the desktop QR code.")
        }
    }

    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        if session.isRunning {
            session.stopRunning()
        }
    }

    private func configureAndStart() {
        guard !configured else {
            if !session.isRunning { session.startRunning() }
            return
        }
        guard let camera = AVCaptureDevice.default(for: .video),
              let input = try? AVCaptureDeviceInput(device: camera),
              session.canAddInput(input) else {
            onError("No camera is available to scan the desktop QR code.")
            return
        }
        let output = AVCaptureMetadataOutput()
        guard session.canAddOutput(output) else {
            onError("This camera cannot scan QR codes.")
            return
        }
        session.addInput(input)
        session.addOutput(output)
        output.setMetadataObjectsDelegate(self, queue: .main)
        output.metadataObjectTypes = [.qr]

        let preview = AVCaptureVideoPreviewLayer(session: session)
        preview.videoGravity = .resizeAspectFill
        preview.frame = view.bounds
        view.layer.insertSublayer(preview, at: 0)
        configured = true
        session.startRunning()
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        (view.layer.sublayers?.first { $0 is AVCaptureVideoPreviewLayer } as? AVCaptureVideoPreviewLayer)?.frame = view.bounds
    }

    func metadataOutput(
        _ output: AVCaptureMetadataOutput,
        didOutput metadataObjects: [AVMetadataObject],
        from connection: AVCaptureConnection
    ) {
        guard !delivered,
              let code = (metadataObjects.first as? AVMetadataMachineReadableCodeObject)?.stringValue else {
            return
        }
        delivered = true
        session.stopRunning()
        onCode(code)
    }
}

private struct PendingAction {
    let pod: PodRow
    let restart: Bool
    var verb: String { restart ? "Restart" : "Delete" }
    var title: String { "\(verb) \(pod.name)?" }
    var message: String {
        restart
            ? "Restart deletes the owned pod so its controller can recreate it."
            : "Delete \(pod.namespace)/\(pod.name)."
    }
}
