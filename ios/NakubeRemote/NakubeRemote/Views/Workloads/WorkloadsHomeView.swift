import SwiftUI

struct WorkloadsHomeView: View {
    let host: String
    let context: ClusterContext

    @State private var workloads: [Workload]?
    @State private var query = ""
    @State private var errorText: String?
    @State private var loading = false
    @State private var refreshID = UUID()
    @State private var requestID = UUID()

    private var filteredWorkloads: [Workload] {
        guard let workloads else { return [] }
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return workloads }
        return workloads.filter { $0.searchText.localizedStandardContains(trimmed) }
    }


    var body: some View {
        List {
            if let errorText {
                Section {
                    ErrorMessageView(title: "Couldn’t load workloads", message: errorText)
                    Button("Try Again") { refreshID = UUID() }
                }
            }
            if loading && workloads == nil {
                Section {
                    ForEach(0..<6, id: \.self) { _ in ContextRowSkeleton() }
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel("Loading workloads")
            } else if let workloads {
                let summary = WorkloadSummary(workloads)
                Section("Summary") {
                    LabeledContent("Total", value: String(summary.total))
                    LabeledContent("Healthy", value: String(summary.healthy))
                    LabeledContent("Active", value: String(summary.active))
                    LabeledContent("Attention", value: String(summary.attention))
                    LabeledContent("Suspended", value: String(summary.suspended))
                    VStack(alignment: .leading, spacing: 8) {
                        LabeledContent("Pod readiness", value: summary.desired > 0 ? "\(summary.ready)/\(summary.desired)" : "—")
                        if summary.desired > 0 {
                            ProgressView(value: min(Double(summary.ready) / Double(summary.desired), 1))
                        }
                    }
                }
                Section("Kinds") {
                    ForEach(WorkloadResource.allCases) { resource in
                        NavigationLink {
                            WorkloadResourceView(host: host, context: context, resource: resource)
                        } label: {
                            Label {
                                LabeledContent(resource.title, value: String(summary.byResource[resource.rawValue, default: 0]))
                            } icon: {
                                Image(systemName: resource.icon)
                            }
                        }
                    }
                }
                Section("Inventory") {
                    if workloads.isEmpty {
                        ContentUnavailableView(
                            "No Workloads",
                            systemImage: "square.3.layers.3d",
                            description: Text("This cluster did not return any workload resources.")
                        )
                    } else if filteredWorkloads.isEmpty {
                        ContentUnavailableView.search(text: query)
                    } else {
                        ForEach(filteredWorkloads) { workload in
                            NavigationLink {
                                WorkloadDetailView(host: host, context: context, workload: workload)
                            } label: {
                                WorkloadRow(workload: workload)
                            }
                        }
                    }
                }
            }
        }
        .navigationTitle("Workloads")
        .navigationBarTitleDisplayMode(.inline)
        .searchable(text: $query, prompt: "Search workloads")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button { refreshID = UUID() } label: {
                    Label("Refresh Workloads", systemImage: "arrow.clockwise")
                }
                .disabled(loading)
            }
        }
        .refreshable { await refresh() }
        .task(id: refreshID) { await refresh() }
        .onDisappear { requestID = UUID() }
        .accessibilityIdentifier("workloads.home")
    }

    @MainActor
    private func refresh() async {
        let id = UUID()
        requestID = id
        loading = true
        errorText = nil
        defer { if requestID == id { loading = false } }
        do {
            let result = try await DesktopAPI(host: host).workloads(context: context.name)
            try Task.checkCancellation()
            guard requestID == id else { return }
            workloads = result
        } catch {
            guard !Task.isCancelled, requestID == id else { return }
            errorText = connectionMessage(error)
        }
    }
}

private struct WorkloadSummary {
    var total = 0
    var healthy = 0
    var active = 0
    var attention = 0
    var suspended = 0
    var ready = 0
    var desired = 0
    var byResource: [String: Int] = [:]

    init(_ workloads: [Workload]) {
        for workload in workloads {
            total += 1
            if workload.resource == WorkloadResource.pods.rawValue {
                ready += workload.ready
                desired += workload.desired
            }
            byResource[workload.resource, default: 0] += 1
            if workload.healthy {
                healthy += 1
            } else if workload.status == "Active" {
                active += 1
            } else if workload.status == "Suspended" {
                suspended += 1
            } else {
                attention += 1
            }
        }
    }
}
