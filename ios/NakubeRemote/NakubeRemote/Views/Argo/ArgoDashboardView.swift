import SwiftUI

struct ArgoDashboardView: View {
    let host: String
    let context: ClusterContext

    @State private var applications: [ArgoApplication]?
    @State private var applicationSets: [ArgoApplicationSet]?
    @State private var projects: [ArgoProject]?
    @State private var repositories: [ArgoRepository]?
    @State private var clusters: [ArgoCluster]?
    @State private var activity: [ClusterEvent]?
    @State private var loading: Set<SectionKey> = []
    @State private var errors: [SectionKey: String] = [:]
    @State private var refreshID = UUID()
    @State private var requestID = UUID()
    @State private var fleetAction: FleetAction?
    @State private var actionError: String?
    @State private var acting = false

    var body: some View {
        List {
            if let actionError {
                Section { ErrorMessageView(title: "Argo action failed", message: actionError) }
            }
            Section("Status") {
                NavigationLink { ArgoApplicationsView(host: host, context: context) } label: {
                    dashboardRow("Applications", value: applicationSummary, icon: "arrow.triangle.branch", key: .applications)
                }
                NavigationLink { ArgoApplicationSetsView(host: host, context: context) } label: {
                    dashboardRow("ApplicationSets", value: sectionCount(applicationSets, key: .sets), icon: "square.stack.3d.up", key: .sets)
                }
                NavigationLink { ArgoProjectsView(host: host, context: context) } label: {
                    dashboardRow("AppProjects", value: sectionCount(projects, key: .projects), icon: "folder", key: .projects)
                }
                NavigationLink { ArgoRepositoriesView(host: host, context: context) } label: {
                    dashboardRow("Repositories", value: sectionCount(repositories, key: .repositories), icon: "externaldrive", key: .repositories)
                }
                NavigationLink { ArgoClustersView(host: host, context: context) } label: {
                    dashboardRow("Clusters", value: sectionCount(clusters, key: .clusters), icon: "globe", key: .clusters)
                }
            }
            if let applications {
                Section("Application health") {
                    LabeledContent("Synced", value: String(applications.filter { $0.sync == "Synced" }.count))
                    LabeledContent("Out of sync", value: String(applications.filter { $0.sync == "OutOfSync" }.count))
                    LabeledContent("Healthy", value: String(applications.filter { $0.health == "Healthy" }.count))
                    LabeledContent("Degraded", value: String(applications.filter { $0.health == "Degraded" }.count))
                    LabeledContent("Missing", value: String(applications.filter { $0.health == "Missing" }.count))
                    LabeledContent("Progressing", value: String(applications.filter { $0.health == "Progressing" }.count))
                }
                Section("Needs attention") {
                    if attention.isEmpty {
                        Text("All Applications are synced and healthy.").foregroundStyle(.secondary)
                    } else {
                        ForEach(attention) { item in
                            VStack(alignment: .leading, spacing: 3) {
                                HStack {
                                    Text(item.name).font(.headline)
                                    Spacer()
                                    Text(item.reason).font(.caption.weight(.semibold)).foregroundStyle(item.color)
                                }
                                Text(item.namespace).font(.subheadline).foregroundStyle(.secondary)
                                Text("Sync: \(shown(item.sync)) · Health: \(shown(item.health))")
                                    .font(.footnote).foregroundStyle(.secondary)
                            }
                            .padding(.vertical, 2)
                            .accessibilityElement(children: .combine)
                        }
                    }
                }
            } else if let error = errors[.applications] {
                Section("Needs attention") { ErrorMessageView(title: "Applications unavailable", message: error) }
            }
            Section("Recent activity") {
                if loading.contains(.activity) {
                    ProgressView("Loading activity")
                } else if let error = errors[.activity] {
                    ErrorMessageView(title: "Activity unavailable", message: error)
                } else if let activity, activity.isEmpty {
                    Text("No Argo events").foregroundStyle(.secondary)
                } else if let activity {
                    ForEach(activity.prefix(8)) { event in
                        VStack(alignment: .leading, spacing: 3) {
                            HStack {
                                Label(event.reason.isEmpty ? "Event" : event.reason, systemImage: activityCategory(event).icon)
                                    .font(.headline)
                                    .foregroundStyle(activityCategory(event).color)
                                Spacer()
                                Text(RemoteDate.relative(event.lastSeen, abbreviated: true))
                                    .font(.caption).foregroundStyle(.secondary)
                            }
                            Text(activityObject(event)).font(.subheadline).foregroundStyle(.secondary)
                            if !event.message.isEmpty {
                                Text(event.message).font(.footnote).foregroundStyle(.secondary).lineLimit(3)
                            }
                        }
                        .padding(.vertical, 2)
                        .accessibilityElement(children: .combine)
                    }
                }
            }
        }
        .navigationTitle("Argo CD")
        .navigationBarTitleDisplayMode(.inline)
        .accessibilityIdentifier("argo.dashboard")
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                Menu("Argo actions", systemImage: "ellipsis.circle") {
                    Button("Sync OutOfSync") { fleetAction = .sync }
                    Button("Refresh All") { fleetAction = .refresh }
                }
                .disabled(acting || applications == nil || loading.contains(.applications) || errors[.applications] != nil)
                Button { refreshID = UUID() } label: { Label("Refresh Argo", systemImage: "arrow.clockwise") }
                    .disabled(acting || !loading.isEmpty)
            }
        }
        .confirmationDialog(fleetAction?.title ?? "", isPresented: Binding(
            get: { fleetAction != nil }, set: { if !$0 { fleetAction = nil } }
        ), titleVisibility: .visible) {
            if fleetAction == .sync {
                Button("Sync") { Task { await runFleet(.sync, prune: false) } }
                Button("Sync and Prune", role: .destructive) { Task { await runFleet(.sync, prune: true) } }
            } else {
                Button("Refresh All") { Task { await runFleet(.refresh, prune: false) } }
            }
            Button("Cancel", role: .cancel) {}
        } message: { Text(fleetAction?.message ?? "") }
        .refreshable { await load() }
        .task(id: refreshID) { await load() }
        .onDisappear { requestID = UUID() }
    }

    @ViewBuilder
    private func dashboardRow(_ title: String, value: String, icon: String, key: SectionKey) -> some View {
        HStack(spacing: 12) {
            Image(systemName: icon).foregroundStyle(.tint).frame(width: 24)
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                if let error = errors[key] { Text(error).font(.caption).foregroundStyle(.red).lineLimit(2) }
            }
            Spacer()
            if loading.contains(key) { ProgressView().controlSize(.small) }
            else { Text(value).foregroundStyle(.secondary).monospacedDigit() }
        }
        .accessibilityElement(children: .combine)
    }

    private var applicationSummary: String {
        guard let applications else { return errors[.applications] == nil ? "Loading" : "Unavailable" }
        let drifted = applications.filter { $0.sync == "OutOfSync" }.count
        return drifted == 0 ? String(applications.count) : "\(drifted) out of sync"
    }

    private func sectionCount<T>(_ values: [T]?, key: SectionKey) -> String {
        if let values { return String(values.count) }
        return errors[key] == nil ? "Loading" : "Unavailable"
    }

    private var attention: [AttentionItem] {
        guard let applications else { return [] }
        let mapped = applications.map(AttentionItem.init)
        return mapped.filter { $0.health == "Degraded" }
            + mapped.filter { $0.health == "Missing" }
            + mapped.filter { $0.health != "Degraded" && $0.health != "Missing" && $0.sync == "OutOfSync" }
    }

    @MainActor private func load() async {
        let id = UUID()
        requestID = id
        errors = [:]
        loading = Set(SectionKey.allCases)
        applications = nil
        applicationSets = nil
        projects = nil
        repositories = nil
        clusters = nil
        activity = nil
        async let applicationsLoad: Void = loadApplications(id)
        async let setsLoad: Void = loadSets(id)
        async let projectsLoad: Void = loadProjects(id)
        async let repositoriesLoad: Void = loadRepositories(id)
        async let clustersLoad: Void = loadClusters(id)
        async let activityLoad: Void = loadActivity(id)
        _ = await (applicationsLoad, setsLoad, projectsLoad, repositoriesLoad, clustersLoad, activityLoad)
    }

    @MainActor private func loadApplications(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).argoApplications(context: context.name)
            if requestID == id { applications = value }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.applications] = connectionMessage(error) }
        }
        finish(.applications, id: id)
    }

    @MainActor private func loadSets(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).argoApplicationSets(context: context.name)
            if requestID == id { applicationSets = value }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.sets] = connectionMessage(error) }
        }
        finish(.sets, id: id)
    }

    @MainActor private func loadProjects(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).argoProjects(context: context.name)
            if requestID == id { projects = value }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.projects] = connectionMessage(error) }
        }
        finish(.projects, id: id)
    }

    @MainActor private func loadRepositories(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).argoRepositories(context: context.name)
            if requestID == id { repositories = value }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.repositories] = connectionMessage(error) }
        }
        finish(.repositories, id: id)
    }

    @MainActor private func loadClusters(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).argoClusters(context: context.name)
            if requestID == id { clusters = value }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.clusters] = connectionMessage(error) }
        }
        finish(.clusters, id: id)
    }

    @MainActor private func loadActivity(_ id: UUID) async {
        do {
            let value = try await DesktopAPI(host: host).events(context: context.name)
            if requestID == id { activity = value.filter(isArgoEvent) }
        } catch {
            if !Task.isCancelled, requestID == id { errors[.activity] = connectionMessage(error) }
        }
        finish(.activity, id: id)
    }

    @MainActor private func finish(_ key: SectionKey, id: UUID) {
        if requestID == id { loading.remove(key) }
    }

    @MainActor private func runFleet(_ action: FleetAction, prune: Bool) async {
        fleetAction = nil
        acting = true
        actionError = nil
        defer { acting = false }
        do {
            let api = DesktopAPI(host: host)
            switch action {
            case .sync: try await api.syncOutOfSyncArgoApplications(context: context.name, prune: prune)
            case .refresh: try await api.refreshAllArgoApplications(context: context.name)
            }
            refreshID = UUID()
        } catch { actionError = connectionMessage(error) }
    }

    private func isArgoEvent(_ event: ClusterEvent) -> Bool {
        let kind = event.object.split(separator: "/", maxSplits: 1).first.map(String.init) ?? event.object
        return kind == "Application" || kind == "ApplicationSet" || kind == "AppProject" || event.apiVersion.contains("argoproj.io")
    }

    private func activityObject(_ event: ClusterEvent) -> String {
        let parts = event.object.split(separator: "/", maxSplits: 1).map(String.init)
        guard parts.count == 2, !event.namespace.isEmpty else { return event.object }
        return "\(parts[0]) \(event.namespace)/\(parts[1])"
    }

    private func activityCategory(_ event: ClusterEvent) -> ActivityCategory {
        let reason = event.reason.lowercased()
        if event.type.caseInsensitiveCompare("Warning") == .orderedSame || reason.contains("error") || reason.contains("fail") { return .failure }
        if reason.contains("start") || reason.contains("progress") || reason.contains("running") { return .progress }
        if reason.contains("completed") || reason.contains("succeed") || reason.contains("healthy") || reason.contains("synced") || reason.contains("updated") { return .success }
        return .info
    }

    private func shown(_ value: String) -> String { value.isEmpty ? "Unknown" : value }

    private enum SectionKey: String, CaseIterable { case applications, sets, projects, repositories, clusters, activity }
    private enum FleetAction: Equatable {
        case sync, refresh
        var title: String { self == .sync ? "Sync OutOfSync applications?" : "Refresh all applications?" }
        var message: String { self == .sync ? "Only applications currently reported OutOfSync are synced." : "Argo CD will compare every application again." }
    }
    private enum ActivityCategory {
        case failure, progress, success, info
        var icon: String {
            switch self { case .failure: "xmark.octagon.fill"; case .progress: "clock.fill"; case .success: "checkmark.circle.fill"; case .info: "info.circle.fill" }
        }
        var color: Color {
            switch self { case .failure: .red; case .progress: .orange; case .success: .green; case .info: .gray }
        }
    }
    private struct AttentionItem: Identifiable {
        let namespace: String
        let name: String
        let sync: String
        let health: String
        var id: String { "\(namespace)/\(name)" }
        init(_ application: ArgoApplication) {
            namespace = application.namespace
            name = application.name
            sync = application.sync
            health = application.health
        }
        var reason: String {
            if health == "Degraded" { return "Degraded" }
            if health == "Missing" { return "Missing" }
            return "Out of sync"
        }
        var color: Color { health == "Degraded" || health == "Missing" ? .red : .orange }
    }
}
