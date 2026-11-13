import SwiftUI

struct PodLogsView: View {
    let host: String
    let context: ClusterContext
    let pod: Workload

    @State private var selectedContainer: String
    @State private var lines: [LogLine] = []
    @State private var nextLineID = 0
    @State private var errorText: String?
    @State private var connecting = false
    @State private var streamID = UUID()

    init(host: String, context: ClusterContext, pod: Workload) {
        self.host = host
        self.context = context
        self.pod = pod
        _selectedContainer = State(initialValue: Self.preferredContainer(pod.containers))
    }

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 2) {
                    if let errorText {
                        ErrorMessageView(title: "Log stream ended", message: errorText)
                            .padding()
                    }
                    if connecting && lines.isEmpty {
                        ProgressView("Connecting to \(selectedContainer)…")
                            .frame(maxWidth: .infinity, minHeight: 160)
                    } else if lines.isEmpty && errorText == nil {
                        ContentUnavailableView(
                            "No Log Lines",
                            systemImage: "doc.plaintext",
                            description: Text("The container has not produced output yet.")
                        )
                        .frame(minHeight: 240)
                    } else {
                        ForEach(lines) { line in
                            Text(line.text)
                                .font(.system(.caption, design: .monospaced))
                                .textSelection(.enabled)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .id(line.id)
                        }
                    }
                }
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
            }
            .background(Color(.systemBackground))
            .onChange(of: lines.count) {
                guard let last = lines.last else { return }
                proxy.scrollTo(last.id, anchor: .bottom)
            }
        }
        .navigationTitle("Logs")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .principal) {
                VStack(spacing: 1) {
                    Text(pod.name).font(.headline)
                    Text(selectedContainer).font(.caption).foregroundStyle(.secondary)
                }
            }
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    Picker("Container", selection: $selectedContainer) {
                        ForEach(pod.containers) { container in
                            Text(container.name).tag(container.name)
                        }
                    }
                    Divider()
                    Button {
                        reconnect(clear: false)
                    } label: {
                        Label("Reconnect", systemImage: "arrow.clockwise")
                    }
                    Button {
                        reconnect(clear: true)
                    } label: {
                        Label("Clear and Reconnect", systemImage: "trash")
                    }
                } label: {
                    Label("Log Options", systemImage: "ellipsis.circle")
                }
            }
        }
        .task(id: streamID) { await stream() }
        .onChange(of: selectedContainer) { reconnect(clear: true) }
        .accessibilityIdentifier("workloads.pod.logs")
    }

    @MainActor
    private func stream() async {
        guard !selectedContainer.isEmpty else {
            errorText = "This pod has no regular containers."
            return
        }
        connecting = true
        errorText = nil
        defer { connecting = false }
        do {
            try await DesktopAPI(host: host).streamLogs(
                context: context.name,
                namespace: pod.namespace,
                pod: pod.name,
                container: selectedContainer
            ) { line in
                append(line)
            }
        } catch {
            guard !Task.isCancelled else { return }
            errorText = connectionMessage(error)
        }
    }

    @MainActor
    private func append(_ text: String) {
        lines.append(LogLine(id: nextLineID, text: text))
        nextLineID += 1
        if lines.count > 1_000 {
            lines.removeFirst(lines.count - 1_000)
        }
    }

    private func reconnect(clear: Bool) {
        if clear {
            lines.removeAll(keepingCapacity: true)
            nextLineID = 0
        }
        streamID = UUID()
    }

    private static func preferredContainer(_ containers: [WorkloadContainer]) -> String {
        let sidecars = Set(["istio-proxy", "linkerd-proxy", "envoy", "vault-agent"])
        return containers.first(where: { !sidecars.contains($0.name) })?.name ?? containers.first?.name ?? ""
    }
}

private struct LogLine: Identifiable {
    let id: Int
    let text: String
}
