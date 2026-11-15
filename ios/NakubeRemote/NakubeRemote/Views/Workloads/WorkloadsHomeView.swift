import SwiftUI

struct WorkloadsHomeView: View {
    let host: String
    let context: ClusterContext

    @State private var workloads: [Workload]?
    // Derived once per load rather than on every render, which includes each search keystroke.
    @State private var summary: WorkloadSummary?
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
            } else if let workloads, let summary {
                let visibleWorkloads = filteredWorkloads
                Section("Summary") {
                    // One glanceable grid instead of six stacked rows, so the inventory stays near the top.
                    Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 14) {
                        GridRow {
                            stat("Total", summary.total, color: .primary)
                            stat("Healthy", summary.healthy, color: .green)
                            stat("Attention", summary.attention, color: summary.attention > 0 ? .orange : .secondary)
                        }
                        GridRow {
                            stat("Active", summary.active, color: .primary)
                            stat("Suspended", summary.suspended, color: .primary)
                            readiness(summary)
                        }
                    }
                    .padding(.vertical, 6)
                }
                Section("Kinds") {
                    ForEach(WorkloadResource.allCases) { resource in
                        NavigationLink {
                            WorkloadResourceView(host: host, context: context, resource: resource)
                        } label: {
                            Label {
                                LabeledContent(resource.title, value: String(summary.byResource[resource.rawValue, default: 0]))
                                    .monospacedDigit()
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
                            systemImage: "square.grid.2x2",
                            description: Text("This cluster did not return any workload resources.")
                        )
                    } else if visibleWorkloads.isEmpty {
                        ContentUnavailableView.search(text: query)
                    } else {
                        ForEach(visibleWorkloads) { workload in
                            NavigationLink {
                                WorkloadDetailView(host: host, context: context, workload: workload) {
                                    refreshID = UUID()
                                }
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

    private func stat(_ title: String, _ value: Int, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(value, format: .number)
                .font(.title2.weight(.semibold)).monospacedDigit()
                .foregroundStyle(color)
            Text(title).font(.footnote).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }

    private func readiness(_ summary: WorkloadSummary) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(summary.desired > 0 ? "\(summary.ready)/\(summary.desired)" : "—")
                .font(.title2.weight(.semibold)).monospacedDigit()
            Text("Pods ready").font(.footnote).foregroundStyle(.secondary)
            if summary.desired > 0 {
                ProgressView(value: min(Double(summary.ready) / Double(summary.desired), 1))
                    .tint(summary.ready < summary.desired ? .orange : .green)
                    .accessibilityHidden(true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
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
            summary = WorkloadSummary(result)
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
