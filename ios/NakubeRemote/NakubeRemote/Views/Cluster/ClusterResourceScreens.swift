import SwiftUI

struct ClusterNodesView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Nodes",
            emptyTitle: "No Nodes",
            emptyMessage: "This cluster did not return any nodes.",
            emptyIcon: "server.rack",
            load: { try await DesktopAPI(host: host).nodes(context: context.name) },
            rowContent: { NodeRow(node: $0) },
            detailContent: { item, _ in NodeDetailView(node: item) }
        )
        .accessibilityIdentifier("cluster.nodes")
    }
}

struct ClusterNamespacesView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Namespaces",
            emptyTitle: "No Namespaces",
            emptyMessage: "This cluster did not return any namespaces.",
            emptyIcon: "square.stack.3d.up",
            load: { try await DesktopAPI(host: host).namespaces(context: context.name) },
            rowContent: { NamespaceRow(namespace: $0) },
            detailContent: { item, _ in NamespaceDetailView(namespace: item) }
        )
        .accessibilityIdentifier("cluster.namespaces")
    }
}

struct ClusterEventsView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Events",
            emptyTitle: "No Events",
            emptyMessage: "No Kubernetes events are currently available.",
            emptyIcon: "bolt.horizontal.circle",
            load: { try await DesktopAPI(host: host).events(context: context.name) },
            rowContent: { EventRow(event: $0) },
            detailContent: { item, _ in EventDetailView(event: item) }
        )
        .accessibilityIdentifier("cluster.events")
    }
}

struct ClusterLeasesView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        ClusterListView(
            title: "Leases",
            emptyTitle: "No Leases",
            emptyMessage: "This cluster did not return any coordination leases.",
            emptyIcon: "timer",
            load: { try await DesktopAPI(host: host).leases(context: context.name) },
            rowContent: { LeaseRow(lease: $0) },
            detailContent: { item, _ in LeaseDetailView(lease: item) }
        )
        .accessibilityIdentifier("cluster.leases")
    }
}

struct ClusterPortForwardsView: View {
    let host: String

    var body: some View {
        ClusterListView(
            title: "Port-forwards",
            emptyTitle: "No Port-forwards",
            emptyMessage: "Start a port-forward from a Pod or Service on the desktop.",
            emptyIcon: "arrow.left.arrow.right",
            load: { try await DesktopAPI(host: host).portForwards() },
            rowContent: { PortForwardRow(forward: $0) },
            detailContent: { item, _ in PortForwardDetailView(forward: item) }
        )
        .accessibilityIdentifier("cluster.port-forwards")
    }
}

private struct NodeRow: View {
    let node: ClusterNode

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .firstTextBaseline) {
                Text(node.name).font(.headline)
                    .lineLimit(2).truncationMode(.middle)
                Spacer(minLength: 8)
                StatusBadge(text: node.ready ? "Ready" : "Not ready", tone: node.ready ? .ok : .warning)
            }
            // Managed clusters (EKS, GKE) leave roles empty, so show the version instead of "No role" on every row.
            Text(node.roles.isEmpty ? node.version : node.roles.joined(separator: ", "))
                .font(.subheadline).foregroundStyle(.secondary).lineLimit(1)
            Text("CPU \(ClusterFormat.percent(node.cpu)) · Memory \(ClusterFormat.percent(node.memory))")
                .font(.footnote).foregroundStyle(.secondary).monospacedDigit()
        }
        .padding(.vertical, 2)
    }
}

private struct NodeDetailView: View {
    let node: ClusterNode

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Ready", value: node.ready ? "Yes" : "No")
                LabeledContent("Roles", value: node.roles.isEmpty ? "None" : node.roles.joined(separator: ", "))
                ValueRow("Version", value: node.version.isEmpty ? "Unknown" : node.version)
                LabeledContent("Created", value: RemoteDate.relative(node.createdAt))
            }
            Section("Utilization") {
                ResourceReadingView(title: "CPU", reading: node.cpu, unit: .cpu)
                ResourceReadingView(title: "Memory", reading: node.memory, unit: .memory)
            }
            if !node.conditions.isEmpty {
                Section("Conditions") {
                    ForEach(node.conditions, id: \.self) { Text($0) }
                }
            }
            if !node.taints.isEmpty {
                Section("Taints") {
                    ForEach(node.taints, id: \.self) { Text($0).textSelection(.enabled) }
                }
            }
        }
        .navigationTitle(node.name)
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct NamespaceRow: View {
    let namespace: ClusterNamespace

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            Text(namespace.name).font(.headline)
                .lineLimit(2).truncationMode(.middle)
            Spacer(minLength: 8)
            Text(RemoteDate.relative(namespace.createdAt, abbreviated: true))
                .font(.footnote).foregroundStyle(.secondary)
                .fixedSize()
            StatusBadge(text: namespace.phase, tone: namespace.phase == "Active" ? .ok : .warning)
        }
        .padding(.vertical, 2)
    }
}

private struct NamespaceDetailView: View {
    let namespace: ClusterNamespace

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Phase", value: namespace.phase)
                LabeledContent("Created", value: RemoteDate.relative(namespace.createdAt))
            }
            if !namespace.labels.isEmpty {
                Section("Labels") {
                    ForEach(namespace.labels.keys.sorted(), id: \.self) { key in
                        ValueRow(key, value: namespace.labels[key] ?? "")
                    }
                }
            }
            if !namespace.finalizers.isEmpty {
                Section("Finalizers") {
                    ForEach(namespace.finalizers, id: \.self) { Text($0).textSelection(.enabled) }
                }
            }
        }
        .navigationTitle(namespace.name)
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct EventRow: View {
    let event: ClusterEvent

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            // Event header: what happened on the left, when on the right.
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                if event.type == "Warning" {
                    StatusBadge(text: "Warning", tone: .warning)
                }
                Text(event.reason.isEmpty ? "Event" : event.reason).font(.headline).lineLimit(1)
                Spacer(minLength: 8)
                Text(timeText)
                    .font(.footnote).foregroundStyle(.secondary).monospacedDigit()
                    .fixedSize()
            }
            Text(subject)
                .font(.subheadline).foregroundStyle(.secondary)
                .lineLimit(1).truncationMode(.middle)
            if !event.message.isEmpty {
                Text(event.message)
                    .font(.footnote).foregroundStyle(.secondary).lineLimit(2)
            }
        }
        .padding(.vertical, 2)
    }

    private var subject: String {
        [event.namespace, event.object].filter { !$0.isEmpty }.joined(separator: " · ")
    }

    private var timeText: String {
        let time = RemoteDate.relative(event.lastSeen, abbreviated: true)
        return event.count > 1 ? "×\(event.count) · \(time)" : time
    }
}

private struct EventDetailView: View {
    let event: ClusterEvent

    var body: some View {
        List {
            Section("Event") {
                LabeledContent("Type", value: event.type)
                LabeledContent("Reason", value: event.reason.isEmpty ? "Unknown" : event.reason)
                ValueRow("Object", value: event.object.isEmpty ? "Unknown" : event.object)
                LabeledContent("Namespace", value: event.namespace)
                ValueRow("Source", value: event.source.isEmpty ? "Unknown" : event.source)
                LabeledContent("Count", value: String(event.count))
                LabeledContent("First seen", value: RemoteDate.relative(event.firstSeen))
                LabeledContent("Last seen", value: RemoteDate.relative(event.lastSeen))
            }
            Section("Message") {
                Text(event.message.isEmpty ? "No message" : event.message).textSelection(.enabled)
            }
        }
        .navigationTitle(event.reason.isEmpty ? "Event" : event.reason)
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct LeaseRow: View {
    let lease: ClusterLease

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(lease.name).font(.headline)
                    .lineLimit(2).truncationMode(.middle)
                Spacer(minLength: 8)
                Text(RemoteDate.relative(lease.renewTime, abbreviated: true))
                    .font(.footnote).foregroundStyle(.secondary)
                    .fixedSize()
            }
            Text([lease.namespace, lease.holder.isEmpty ? "No holder" : lease.holder].joined(separator: " · "))
                .font(.subheadline).foregroundStyle(.secondary)
                .lineLimit(1).truncationMode(.middle)
        }
        .padding(.vertical, 2)
    }
}

private struct LeaseDetailView: View {
    let lease: ClusterLease

    var body: some View {
        List {
            Section("Lease") {
                LabeledContent("Namespace", value: lease.namespace)
                ValueRow("Holder", value: lease.holder.isEmpty ? "None" : lease.holder)
                LabeledContent("Renewed", value: RemoteDate.relative(lease.renewTime))
                if let duration = lease.durationSeconds {
                    LabeledContent("Duration", value: "\(duration) seconds")
                }
                if let transitions = lease.transitions {
                    LabeledContent("Transitions", value: String(transitions))
                }
                LabeledContent("Created", value: RemoteDate.relative(lease.createdAt))
            }
        }
        .navigationTitle(lease.name)
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct PortForwardRow: View {
    let forward: ClusterPortForward

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(forward.pod).font(.headline)
                    .lineLimit(2).truncationMode(.middle)
                Spacer(minLength: 8)
                Text("\(forward.local) → \(forward.remote)")
                    .font(.subheadline.weight(.medium)).monospacedDigit()
                    .fixedSize()
            }
            Text(forward.namespace).font(.subheadline).foregroundStyle(.secondary)
                .lineLimit(1).truncationMode(.middle)
        }
        .padding(.vertical, 2)
    }
}

private struct PortForwardDetailView: View {
    let forward: ClusterPortForward

    var body: some View {
        List {
            Section("Port-forward") {
                LabeledContent("Namespace", value: forward.namespace)
                ValueRow("Pod", value: forward.pod)
                LabeledContent("Local", value: "localhost:\(forward.local)")
                LabeledContent("Remote", value: String(forward.remote))
            }
            Section {
                Text("Port-forwards are managed by the desktop process and are read-only on iPhone.")
                    .foregroundStyle(.secondary)
            }
        }
        .navigationTitle(forward.pod)
        .navigationBarTitleDisplayMode(.inline)
    }
}


private struct ResourceReadingView: View {
    enum Unit { case cpu, memory }

    let title: String
    let reading: ResourceReading
    let unit: Unit

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text(title)
                Spacer()
                Text(valueText).foregroundStyle(.secondary).monospacedDigit()
            }
            if let percentage = reading.percentage {
                ProgressView(value: min(percentage, 100), total: 100)
                Text("\(percentage.formatted(.number.precision(.fractionLength(1))))% of allocatable")
                    .font(.caption).foregroundStyle(.secondary)
            } else {
                Text("Utilization unavailable").font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 4)
    }

    private var valueText: String {
        guard let used = reading.used else { return "Unavailable" }
        switch unit {
        case .cpu:
            return "\(used)m / \(reading.allocatable)m"
        case .memory:
            return "\(ClusterFormat.bytes(used)) / \(ClusterFormat.bytes(reading.allocatable))"
        }
    }
}

private enum ClusterFormat {

    static func percent(_ reading: ResourceReading) -> String {
        guard let percentage = reading.percentage else { return "—" }
        return "\(percentage.formatted(.number.precision(.fractionLength(0))))%"
    }

    static func bytes(_ value: Int64) -> String {
        ByteCountFormatter.string(fromByteCount: value, countStyle: .memory)
    }
}
