import SwiftUI

struct ArgoApplicationsView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Argo Applications",
            emptyTitle: "No Applications",
            emptyMessage: "Argo CD did not return any applications for this context.",
            emptyIcon: "arrow.triangle.branch",
            load: { try await DesktopAPI(host: host).argoApplications(context: context.name) },
            rowContent: { ArgoApplicationRow(application: $0) },
            detailContent: { item, reload in
                ArgoApplicationDetailView(host: host, context: context, application: item, reload: reload)
            }
        )
        .accessibilityIdentifier("argo.applications")
    }
}

private struct ArgoApplicationRow: View {
    let application: ArgoApplication

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack(alignment: .firstTextBaseline) {
                Text(application.name)
                    .font(.headline)
                    .lineLimit(2)
                    .truncationMode(.middle)
                Spacer(minLength: 8)
                StatusBadge(text: display(application.health, fallback: "Unknown health"), tone: application.healthTone)
            }
            Text(application.namespace)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            HStack(spacing: 6) {
                StatusBadge(text: display(application.sync, fallback: "Unknown sync"), tone: application.syncTone)
                if !application.project.isEmpty {
                    Text(application.project)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
        }
        .padding(.vertical, 2)
        .accessibilityElement(children: .combine)
    }
}

private struct ArgoApplicationDetailView: View {
    let host: String
    let context: ClusterContext
    let application: ArgoApplication
    let reload: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var revision = ""
    @State private var prune = false
    @State private var pendingAction: PendingAction?
    @State private var actionError: String?
    @State private var acting = false

    var body: some View {
        List {
            if let actionError {
                Section { ErrorMessageView(title: "Argo action failed", message: actionError) }
            }
            Section("Actions") {
                TextField("Revision", text: $revision)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                Toggle("Prune resources", isOn: $prune)
                Button("Sync Application") { pendingAction = .sync }
                Button("Refresh") { pendingAction = .refresh }
                Button("Hard Refresh") { pendingAction = .hardRefresh }
                Button("Delete Application", role: .destructive) { pendingAction = .delete }
            }
            .disabled(acting)
            Section("Status") {
                LabeledContent("Health") {
                    StatusBadge(text: display(application.health, fallback: "Unknown"), tone: application.healthTone)
                }
                LabeledContent("Sync") {
                    StatusBadge(text: display(application.sync, fallback: "Unknown"), tone: application.syncTone)
                }
                if !application.healthMessage.isEmpty {
                    ValueRow("Health message", value: application.healthMessage)
                }
                if !application.conditions.isEmpty {
                    ForEach(application.conditions, id: \.self) { condition in
                        Text(condition).textSelection(.enabled)
                    }
                }
            }

            Section("Application") {
                ValueRow("Namespace", value: display(application.namespace))
                ValueRow("Project", value: display(application.project))
                ValueRow("Policy", value: display(application.policy))
                ValueRow("Destination", value: display(application.destination))
                ValueRow("Created", value: RemoteDate.relative(application.createdAt))
            }

            Section("Revision") {
                ValueRow("Current", value: display(application.revision))
                ValueRow("Target", value: display(application.targetRevision))
                ValueRow("Source type", value: display(application.sourceType))
            }

            if !application.operationPhase.isEmpty || !application.operationMessage.isEmpty || !application.operationBy.isEmpty {
                Section("Last operation") {
                    if !application.operationPhase.isEmpty {
                        ValueRow("Phase", value: application.operationPhase)
                    }
                    if !application.operationBy.isEmpty {
                        ValueRow("Initiated by", value: application.operationBy)
                    }
                    if !application.operationMessage.isEmpty {
                        ValueRow("Message", value: application.operationMessage)
                    }
                }
            }

            if !application.sources.isEmpty {
                Section("Sources") {
                    ForEach(Array(application.sources.enumerated()), id: \.offset) { _, source in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(source.repoURL.isEmpty ? "Source" : source.repoURL)
                                .font(.headline)
                                .textSelection(.enabled)
                            if !source.path.isEmpty { Text("Path: \(source.path)") }
                            if !source.chart.isEmpty { Text("Chart: \(source.chart)") }
                            if !source.targetRevision.isEmpty { Text("Target: \(source.targetRevision)") }
                            if !source.ref.isEmpty { Text("Reference: \(source.ref)") }
                        }
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .padding(.vertical, 2)
                        .accessibilityElement(children: .combine)
                    }
                }
            }

            if !application.resources.isEmpty {
                Section("Resources") {
                    ForEach(application.resources) { resource in
                        VStack(alignment: .leading, spacing: 5) {
                            Text(resourceTitle(resource)).font(.headline)
                            Text(resourceLocation(resource))
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                            HStack(spacing: 8) {
                                if !resource.sync.isEmpty {
                                    Text(resource.sync)
                                }
                                if !resource.health.isEmpty {
                                    Text(resource.health)
                                }
                                if resource.hook {
                                    Text("Hook")
                                }
                            }
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        }
                        .padding(.vertical, 2)
                        .accessibilityElement(children: .combine)
                    }
                }
            }

            if !application.images.isEmpty {
                Section("Images") {
                    ForEach(application.images, id: \.self) { image in
                        Text(image).textSelection(.enabled)
                    }
                }
            }

            if !application.history.isEmpty {
                Section("History") {
                    ForEach(application.history) { item in
                        Button { pendingAction = .rollback(item.id) } label: {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(item.revision.isEmpty ? "Revision \(item.id)" : item.revision)
                                    .font(.headline)
                                    .textSelection(.enabled)
                                Text(historyDescription(item))
                                    .font(.subheadline)
                                    .foregroundStyle(.secondary)
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.vertical, 2)
                        }
                        .buttonStyle(.plain)
                        .accessibilityHint("Roll back to this revision")
                        .disabled(acting)
                    }
                }
            }
        }
        .navigationTitle(application.name)
        .navigationBarTitleDisplayMode(.inline)
        .accessibilityIdentifier("argo.application.detail")
        .task {
            if revision.isEmpty { revision = application.targetRevision }
        }
        .confirmationDialog(actionTitle, isPresented: Binding(
            get: { pendingAction != nil },
            set: { if !$0 { pendingAction = nil } }
        ), titleVisibility: .visible) {
            switch pendingAction {
            case .sync:
                Button("Sync") { Task { await perform(.sync) } }
            case .refresh:
                Button("Refresh") { Task { await perform(.refresh) } }
            case .hardRefresh:
                Button("Hard Refresh", role: .destructive) { Task { await perform(.hardRefresh) } }
            case .rollback(let id):
                Button("Roll Back", role: .destructive) { Task { await perform(.rollback(id)) } }
            case .delete:
                Button("Delete Foreground", role: .destructive) { Task { await perform(.deleteCascade("foreground")) } }
                Button("Delete Background", role: .destructive) { Task { await perform(.deleteCascade("background")) } }
                Button("Delete Without Cascading", role: .destructive) { Task { await perform(.deleteCascade("non-cascading")) } }
            case nil, .deleteCascade(_):
                EmptyView()
            }
            Button("Cancel", role: .cancel) {}
        }
    }

    private var actionTitle: String {
        switch pendingAction {
        case .sync: "Sync \(application.name)?"
        case .refresh: "Refresh \(application.name)?"
        case .hardRefresh: "Hard refresh \(application.name)?"
        case .rollback: "Roll back \(application.name)?"
        case .delete: "Delete \(application.name)?"
        case .deleteCascade(_), nil: ""
        }
    }

    @MainActor
    private func perform(_ action: PendingAction) async {
        pendingAction = nil
        acting = true
        actionError = nil
        defer { acting = false }
        do {
            let api = DesktopAPI(host: host)
            switch action {
            case .sync:
                try await api.syncArgoApplication(
                    context: context.name, namespace: application.namespace, name: application.name,
                    revision: revision.trimmingCharacters(in: .whitespacesAndNewlines), prune: prune
                )
            case .refresh:
                try await api.refreshArgoApplication(context: context.name, namespace: application.namespace, name: application.name, hard: false)
            case .hardRefresh:
                try await api.refreshArgoApplication(context: context.name, namespace: application.namespace, name: application.name, hard: true)
            case .rollback(let id):
                try await api.rollbackArgoApplication(context: context.name, namespace: application.namespace, name: application.name, id: id, prune: prune)
            case .deleteCascade(let cascade):
                try await api.deleteArgoResource(
                    context: context.name, resource: "applications", namespace: application.namespace,
                    name: application.name, cascade: cascade
                )
                reload()
                dismiss()
                return
            case .delete:
                return
            }
            reload()
            dismiss()
        } catch {
            actionError = connectionMessage(error)
        }
    }

    private enum PendingAction {
        case sync
        case refresh
        case hardRefresh
        case rollback(Int64)
        case delete
        case deleteCascade(String)
    }

    private func resourceTitle(_ resource: ArgoResource) -> String {
        resource.kind.isEmpty ? resource.name : "\(resource.kind) · \(resource.name)"
    }

    private func resourceLocation(_ resource: ArgoResource) -> String {
        resource.namespace.isEmpty ? "Cluster scoped" : resource.namespace
    }

    private func historyDescription(_ item: ArgoHistory) -> String {
        item.by.isEmpty ? "History ID \(item.id)" : "History ID \(item.id) · \(item.by)"
    }
}

private func display(_ value: String, fallback: String = "Unknown") -> String {
    value.isEmpty ? fallback : value
}
