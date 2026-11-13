import SwiftUI

struct ClusterOverviewView: View {
    let connection: DesktopConnection
    let context: ClusterContext
    @State private var overview: OverviewBody?
    @State private var errorText: String?
    @State private var loading = false
    @State private var updatedAt: Date?
    @State private var refreshID = UUID()
    @State private var requestID = UUID()

    var body: some View {
        List {
            Section("Context") {
                LabeledContent("Name", value: context.name)
                if !context.cluster.isEmpty { LabeledContent("Cluster", value: context.cluster) }
                LabeledContent("Desktop", value: connection.displayHost)
            }
            if let errorText {
                Section {
                    ErrorMessageView(title: "Couldn’t load overview", message: errorText)
                    Button("Try Again") { refreshID = UUID() }
                        .accessibilityIdentifier("overview.retry")
                }
            }
            if let overview, overview.context == context.name {
                if let charts = overview.charts {
                    Section {
                        ResourceUsageChart(overview: overview, charts: charts)
                    } header: {
                        Text("Resource Usage")
                    } footer: {
                        Text("Current usage compared with allocatable capacity. The dashed line marks 100%.")
                    }
                    Section("Cluster Health") {
                        ClusterHealthChart(
                            title: "Nodes", healthyLabel: "Ready", otherLabel: "Not ready",
                            healthy: charts.nodesReady, total: charts.nodesTotal, color: .green
                        )
                        ClusterHealthChart(
                            title: "Pods", healthyLabel: "Running", otherLabel: "Other phases",
                            healthy: charts.podsRunning, total: charts.podsTotal, color: .blue
                        )
                    }
                    Section("Warnings") {
                        Label(overview.warnings, systemImage: "exclamationmark.triangle")
                    }
                } else {
                    Section("Cluster Overview") {
                        LabeledContent("Nodes", value: overview.nodes)
                        LabeledContent("Pods", value: overview.pods)
                        LabeledContent("Warnings", value: overview.warnings)
                        LabeledContent("CPU", value: overview.cpu)
                        LabeledContent("Memory", value: overview.memory)
                        Text("Update and restart NaKube Desktop to enable charts.")
                            .font(.footnote).foregroundStyle(.secondary)
                    }
                }
                if let updatedAt {
                    Section {
                        Text("Updated \(updatedAt.formatted(date: .omitted, time: .standard))")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
            } else if loading {
                OverviewSkeleton()
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("Loading cluster overview")
            }
        }
        .accessibilityIdentifier("cluster.overview")
        .navigationTitle("Cluster Overview")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button { refreshID = UUID() } label: {
                    Label("Refresh Overview", systemImage: "arrow.clockwise")
                }
                .disabled(loading)
            }
        }
        .refreshable { await refresh() }
        .task(id: refreshID) { await refresh() }
        .onDisappear {
            requestID = UUID()
            overview = nil
            updatedAt = nil
        }
    }

    @MainActor
    private func refresh() async {
        let id = UUID()
        requestID = id
        loading = true
        errorText = nil
        // Clear previous metrics on refresh too: a failed request must not look live.
        overview = nil
        updatedAt = nil
        defer { if requestID == id { loading = false } }
        do {
            let result = try await DesktopAPI(connection: connection).overview(context: context.name)
            try Task.checkCancellation()
            guard requestID == id else { return }
            overview = result
            updatedAt = Date()
        } catch {
            guard !Task.isCancelled, requestID == id else { return }
            errorText = connectionMessage(error)
        }
    }
}
