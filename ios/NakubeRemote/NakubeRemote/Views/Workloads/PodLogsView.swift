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
    @State private var atBottom = true
    @State private var userScrolling = false

    private static let bottomID = "logs.bottom"

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
                    // iOS 17 fallback: lazy stacks only materialize rows near the viewport,
                    // so this sentinel's visibility approximates "the reader is at the tail".
                    // iOS 18 measures the exact offset in LogTailTracking instead.
                    Color.clear
                        .frame(height: 1)
                        .id(Self.bottomID)
                        .onAppear { if #unavailable(iOS 18.0) { atBottom = true } }
                        .onDisappear { if #unavailable(iOS 18.0) { atBottom = false } }
                }
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
            }
            .background(Color(.systemBackground))
            .modifier(LogTailTracking(atBottom: $atBottom, userScrolling: $userScrolling))
            // Watch the newest id, not the count: the count stops changing once the buffer is full.
            .onChange(of: lines.last?.id) {
                // Never move content while the finger owns the scroll view or its fling is
                // still settling; the next line after it comes to rest at the tail catches up.
                guard atBottom, !userScrolling else { return }
                proxy.scrollTo(Self.bottomID, anchor: .bottom)
            }
            .overlay(alignment: .bottom) {
                if !atBottom && !lines.isEmpty {
                    Button {
                        proxy.scrollTo(Self.bottomID, anchor: .bottom)
                    } label: {
                        Label("Jump to Latest", systemImage: "arrow.down")
                            .font(.subheadline.weight(.semibold))
                            .padding(.horizontal, 4)
                            .frame(minHeight: 36)
                    }
                    .buttonStyle(.borderedProminent)
                    .buttonBorderShape(.capsule)
                    .padding(.bottom, 12)
                    .transition(.opacity)
                    .accessibilityIdentifier("workloads.pod.logs.jumpToLatest")
                }
            }
            .animation(.easeOut(duration: 0.2), value: atBottom)
        }
        .navigationTitle("Logs")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .principal) {
                VStack(spacing: 1) {
                    // Pod names differ by their trailing hash, so keep the end visible.
                    Text(pod.name).font(.headline)
                        .lineLimit(1).truncationMode(.middle)
                    Text(selectedContainer).font(.caption).foregroundStyle(.secondary)
                        .lineLimit(1).truncationMode(.middle)
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

/// Reports whether the log is scrolled to its tail and whether the user is driving the scroll.
private struct LogTailTracking: ViewModifier {
    @Binding var atBottom: Bool
    @Binding var userScrolling: Bool

    func body(content: Content) -> some View {
        if #available(iOS 18.0, *) {
            content
                .onScrollGeometryChange(for: Bool.self) { geometry in
                    // A line of slack so a fling that lands a few points short still follows.
                    geometry.visibleRect.maxY >= geometry.contentSize.height - 24
                } action: { _, isAtTail in
                    atBottom = isAtTail
                }
                .onScrollPhaseChange { _, phase in
                    // .animating is our own scrollTo, so only finger-driven phases block following.
                    userScrolling = phase == .tracking || phase == .interacting || phase == .decelerating
                }
        } else {
            content
        }
    }
}

private struct LogLine: Identifiable {
    let id: Int
    let text: String
}
