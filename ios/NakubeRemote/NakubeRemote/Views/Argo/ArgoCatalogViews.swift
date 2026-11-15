import SwiftUI

struct ArgoApplicationSetsView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "ApplicationSets", emptyTitle: "No ApplicationSets",
            emptyMessage: "Argo CD did not return any ApplicationSets.", emptyIcon: "square.stack.3d.up",
            load: { try await DesktopAPI(host: host).argoApplicationSets(context: context.name) },
            rowContent: { ArgoCatalogRow(title: $0.name, subtitle: $0.namespace, detail: $0.generators) },
            detailContent: { item, reload in
                ArgoApplicationSetDetail(host: host, context: context, item: item, reload: reload)
            }
        )
        .accessibilityIdentifier("argo.applicationsets")
    }
}

struct ArgoProjectsView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "AppProjects", emptyTitle: "No AppProjects",
            emptyMessage: "Argo CD did not return any AppProjects.", emptyIcon: "folder",
            load: { try await DesktopAPI(host: host).argoProjects(context: context.name) },
            rowContent: { ArgoCatalogRow(title: $0.name, subtitle: $0.namespace, detail: $0.description) },
            detailContent: { item, reload in
                ArgoProjectDetail(host: host, context: context, item: item, reload: reload)
            }
        )
        .accessibilityIdentifier("argo.projects")
    }
}

struct ArgoRepositoriesView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Repositories", emptyTitle: "No Repositories",
            emptyMessage: "Argo CD did not return any credential-safe repository records.", emptyIcon: "externaldrive",
            load: { try await DesktopAPI(host: host).argoRepositories(context: context.name) },
            rowContent: { ArgoCatalogRow(title: $0.name, subtitle: $0.type, detail: $0.url) },
            detailContent: { item, _ in ArgoRepositoryDetail(item: item) }
        )
        .accessibilityIdentifier("argo.repositories")
    }
}

struct ArgoClustersView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Argo Clusters", emptyTitle: "No Argo Clusters",
            emptyMessage: "Argo CD did not return any credential-safe cluster records.", emptyIcon: "globe",
            load: { try await DesktopAPI(host: host).argoClusters(context: context.name) },
            rowContent: { ArgoCatalogRow(title: $0.name, subtitle: $0.namespace, detail: $0.server) },
            detailContent: { item, _ in ArgoClusterDetail(item: item) }
        )
        .accessibilityIdentifier("argo.clusters")
    }
}

private struct ArgoCatalogRow: View {
    let title: String
    let subtitle: String
    let detail: String

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.headline).lineLimit(2).truncationMode(.middle)
            if !subtitle.isEmpty { Text(subtitle).font(.subheadline).foregroundStyle(.secondary) }
            if !detail.isEmpty { Text(detail).font(.footnote).foregroundStyle(.secondary).lineLimit(2) }
        }
        .padding(.vertical, 2)
        .accessibilityElement(children: .combine)
    }
}

private struct ArgoApplicationSetDetail: View {
    let host: String
    let context: ClusterContext
    let item: ArgoApplicationSet
    let reload: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var confirmsDelete = false
    @State private var deleting = false
    @State private var errorText: String?

    var body: some View {
        List {
            if let errorText { ErrorMessageView(title: "Couldn’t delete ApplicationSet", message: errorText) }
            Section("ApplicationSet") {
                ValueRow("Namespace", value: item.namespace)
                ValueRow("Generators", value: item.generators.isEmpty ? "Unknown" : item.generators)
                ValueRow("Project", value: item.project.isEmpty ? "Unknown" : item.project)
                ValueRow("Destination", value: item.destination.isEmpty ? "Unknown" : item.destination)
                ValueRow("Created", value: RemoteDate.relative(item.createdAt))
            }
            if !item.conditions.isEmpty {
                Section("Conditions") { ForEach(item.conditions, id: \.self) { Text($0).textSelection(.enabled) } }
            }
            Section { Button("Delete ApplicationSet", role: .destructive) { confirmsDelete = true }.disabled(deleting) }
        }
        .navigationTitle(item.name).navigationBarTitleDisplayMode(.inline)
        .confirmationDialog("Delete \(item.name)?", isPresented: $confirmsDelete, titleVisibility: .visible) {
            Button("Delete ApplicationSet", role: .destructive) { Task { await delete() } }
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes the ApplicationSet from \(context.displayName).") }
    }

    @MainActor private func delete() async {
        deleting = true; errorText = nil
        defer { deleting = false }
        do {
            try await DesktopAPI(host: host).deleteArgoResource(context: context.name, resource: "applicationsets", namespace: item.namespace, name: item.name)
            reload(); dismiss()
        } catch { errorText = connectionMessage(error) }
    }
}

private struct ArgoProjectDetail: View {
    let host: String
    let context: ClusterContext
    let item: ArgoProject
    let reload: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var confirmsDelete = false
    @State private var deleting = false
    @State private var errorText: String?

    var body: some View {
        List {
            if let errorText { ErrorMessageView(title: "Couldn’t delete AppProject", message: errorText) }
            Section("AppProject") {
                ValueRow("Namespace", value: item.namespace)
                ValueRow("Description", value: item.description.isEmpty ? "None" : item.description)
                LabeledContent("Roles", value: String(item.roles))
                ValueRow("Created", value: RemoteDate.relative(item.createdAt))
            }
            if !item.sourceRepos.isEmpty { Section("Source repositories") { ForEach(item.sourceRepos, id: \.self) { Text($0).textSelection(.enabled) } } }
            if !item.destinations.isEmpty { Section("Destinations") { ForEach(item.destinations, id: \.self) { Text($0).textSelection(.enabled) } } }
            Section { Button("Delete AppProject", role: .destructive) { confirmsDelete = true }.disabled(deleting) }
        }
        .navigationTitle(item.name).navigationBarTitleDisplayMode(.inline)
        .confirmationDialog("Delete \(item.name)?", isPresented: $confirmsDelete, titleVisibility: .visible) {
            Button("Delete AppProject", role: .destructive) { Task { await delete() } }
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes the AppProject from \(context.displayName).") }
    }

    @MainActor private func delete() async {
        deleting = true; errorText = nil
        defer { deleting = false }
        do {
            try await DesktopAPI(host: host).deleteArgoResource(context: context.name, resource: "appprojects", namespace: item.namespace, name: item.name)
            reload(); dismiss()
        } catch { errorText = connectionMessage(error) }
    }
}

private struct ArgoRepositoryDetail: View {
    let item: ArgoRepository
    var body: some View {
        List { Section("Repository") {
            ValueRow("Namespace", value: item.namespace)
            ValueRow("Type", value: item.type)
            ValueRow("URL", value: item.url)
            ValueRow("Project", value: item.project.isEmpty ? "All projects" : item.project)
        } }
        .navigationTitle(item.name).navigationBarTitleDisplayMode(.inline)
    }
}

private struct ArgoClusterDetail: View {
    let item: ArgoCluster
    var body: some View {
        List { Section("Cluster") {
            ValueRow("Namespace", value: item.namespace)
            ValueRow("Server", value: item.server)
        } }
        .navigationTitle(item.name).navigationBarTitleDisplayMode(.inline)
    }
}
