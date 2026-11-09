import SwiftUI

struct WorkloadResourceView: View {
    let host: String
    let context: ClusterContext
    let resource: WorkloadResource

    var body: some View {
        ClusterListView(
            title: resource.title,
            emptyTitle: "No \(resource.title)",
            emptyMessage: "This cluster did not return any \(resource.title.lowercased()).",
            emptyIcon: resource.icon,
            load: { try await DesktopAPI(host: host).workloads(context: context.name, resource: resource) },
            rowContent: { WorkloadRow(workload: $0, showsKind: false) },
            detailContent: { WorkloadDetailView(host: host, context: context, workload: $0) }
        )
        .accessibilityIdentifier("workloads.\(resource.rawValue)")
    }
}

struct WorkloadRow: View {
    let workload: Workload
    var showsKind = true

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text(workload.name).font(.headline)
                Spacer()
                WorkloadStatusLabel(workload: workload)
            }
            HStack(spacing: 6) {
                if showsKind {
                    Text(workload.kind)
                    Text("·")
                }
                Text(workload.namespace)
                Spacer()
                Text("Ready \(workload.readyText)").monospacedDigit()
            }
            .font(.subheadline)
            .foregroundStyle(.secondary)
            Text(RemoteDate.relative(workload.createdAt))
                .font(.caption).foregroundStyle(.tertiary)
        }
        .padding(.vertical, 3)
    }
}

struct WorkloadDetailView: View {
    let host: String
    let context: ClusterContext
    let workload: Workload

    @Environment(\.dismiss) private var dismiss
    @State private var confirmation: PodAction?
    @State private var actionRunning = false
    @State private var actionError: String?

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Kind", value: workload.kind)
                LabeledContent("Namespace", value: workload.namespace)
                LabeledContent("Status", value: workload.status)
                LabeledContent("Ready", value: workload.readyText)
                LabeledContent("Created", value: RemoteDate.relative(workload.createdAt))
            }
            if !workload.facts.isEmpty {
                Section("Details") {
                    ForEach(workload.facts) { fact in
                        LabeledContent(fact.label, value: fact.value)
                    }
                }
            }
            if !workload.containers.isEmpty {
                Section("Containers") {
                    ForEach(workload.containers) { container in
                        VStack(alignment: .leading, spacing: 5) {
                            HStack {
                                Text(container.name).font(.headline)
                                Spacer()
                                if workload.resource == WorkloadResource.pods.rawValue {
                                    Image(systemName: container.ready ? "checkmark.circle.fill" : "circle")
                                        .foregroundStyle(container.ready ? .green : .secondary)
                                        .accessibilityLabel(container.ready ? "Ready" : "Not ready")
                                }
                            }
                            Text(container.image).font(.caption).foregroundStyle(.secondary).textSelection(.enabled)
                            if workload.resource == WorkloadResource.pods.rawValue {
                                Text("\(container.restarts) restarts")
                                    .font(.caption2).foregroundStyle(.tertiary).monospacedDigit()
                            }
                        }
                        .padding(.vertical, 2)
                    }
                }
            }
            if workload.resource == WorkloadResource.pods.rawValue {
                Section("Pod Actions") {
                    if !workload.containers.isEmpty {
                        NavigationLink {
                            PodLogsView(host: host, context: context, pod: workload)
                        } label: {
                            Label("Logs", systemImage: "doc.plaintext")
                        }
                    }
                    Button {
                        confirmation = .restart
                    } label: {
                        Label("Restart Pod", systemImage: "arrow.clockwise")
                    }
                    .disabled(actionRunning || !workload.owned)
                    Button(role: .destructive) {
                        confirmation = .delete
                    } label: {
                        Label("Delete Pod", systemImage: "trash")
                    }
                    .disabled(actionRunning)
                    if !workload.owned {
                        Text("This bare pod cannot be restarted because no controller can recreate it. Delete remains available.")
                            .font(.footnote).foregroundStyle(.secondary)
                    }
                    if actionRunning {
                        ProgressView("Updating pod…")
                    }
                    if let actionError {
                        ErrorMessageView(title: "Pod action failed", message: actionError)
                    }
                }
            }
        }
        .navigationTitle(workload.name)
        .navigationBarTitleDisplayMode(.inline)
        .alert(item: $confirmation) { action in
            Alert(
                title: Text(action.title),
                message: Text(action.message),
                primaryButton: .destructive(Text(action.buttonTitle)) {
                    Task { await perform(action) }
                },
                secondaryButton: .cancel()
            )
        }
    }

    @MainActor
    private func perform(_ action: PodAction) async {
        actionRunning = true
        actionError = nil
        defer { actionRunning = false }
        do {
            let api = DesktopAPI(host: host)
            switch action {
            case .restart:
                try await api.restartPod(context: context.name, namespace: workload.namespace, name: workload.name)
            case .delete:
                try await api.deletePod(context: context.name, namespace: workload.namespace, name: workload.name)
            }
            dismiss()
        } catch {
            guard !Task.isCancelled else { return }
            actionError = connectionMessage(error)
        }
    }
}

private enum PodAction: String, Identifiable {
    case restart
    case delete

    var id: String { rawValue }
    var title: String { self == .restart ? "Restart Pod?" : "Delete Pod?" }
    var buttonTitle: String { self == .restart ? "Restart" : "Delete" }
    var message: String {
        switch self {
        case .restart:
            "The current pod will be deleted and its controller will recreate it."
        case .delete:
            "The pod will be deleted from the selected cluster. This cannot be undone."
        }
    }
}

private struct WorkloadStatusLabel: View {
    let workload: Workload

    var body: some View {
        let style = self.style
        Label(workload.status, systemImage: style.icon)
            .font(.caption.weight(.medium))
            .foregroundStyle(style.color)
    }

    private var style: (icon: String, color: Color) {
        switch workload.status {
        case "Healthy", "Succeeded", "Scheduled":
            ("checkmark.circle.fill", .green)
        case "Failed", "Unknown":
            ("exclamationmark.triangle.fill", .red)
        default:
            ("clock.fill", .orange)
        }
    }
}
