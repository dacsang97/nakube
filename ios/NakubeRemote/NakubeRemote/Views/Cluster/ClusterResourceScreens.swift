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
            detailContent: { NodeDetailView(node: $0) }
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
            detailContent: { NamespaceDetailView(namespace: $0) }
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
            detailContent: { EventDetailView(event: $0) }
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
            detailContent: { LeaseDetailView(lease: $0) }
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
            detailContent: { PortForwardDetailView(forward: $0) }
        )
        .accessibilityIdentifier("cluster.port-forwards")
    }
}

private struct NodeRow: View {
    let node: ClusterNode

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text(node.name).font(.headline)
                Spacer()
                StatusText(text: node.ready ? "Ready" : "Not ready", healthy: node.ready)
            }
            Text(node.roles.isEmpty ? "No role" : node.roles.joined(separator: ", "))
                .font(.subheadline).foregroundStyle(.secondary)
            Text("CPU \(ClusterFormat.percent(node.cpu)) · Memory \(ClusterFormat.percent(node.memory))")
                .font(.caption).foregroundStyle(.secondary).monospacedDigit()
        }
        .padding(.vertical, 3)
    }
}

private struct NodeDetailView: View {
    let node: ClusterNode

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Ready", value: node.ready ? "Yes" : "No")
                LabeledContent("Roles", value: node.roles.isEmpty ? "None" : node.roles.joined(separator: ", "))
                LabeledContent("Version", value: node.version.isEmpty ? "Unknown" : node.version)
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
        VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text(namespace.name).font(.headline)
                Spacer()
                StatusText(text: namespace.phase, healthy: namespace.phase == "Active")
            }
            Text("\(namespace.labels.count) labels · \(RemoteDate.relative(namespace.createdAt))")
                .font(.caption).foregroundStyle(.secondary)
        }
        .padding(.vertical, 3)
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
                        LabeledContent(key, value: namespace.labels[key] ?? "")
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
        VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text(event.reason.isEmpty ? event.object : event.reason).font(.headline)
                Spacer()
                StatusText(text: event.type, healthy: event.type != "Warning")
            }
            Text(event.object).font(.subheadline).foregroundStyle(.secondary)
            Text(event.message).font(.caption).foregroundStyle(.secondary).lineLimit(2)
            Text("\(event.namespace) · \(RemoteDate.relative(event.lastSeen))")
                .font(.caption2).foregroundStyle(.tertiary)
        }
        .padding(.vertical, 3)
    }
}

private struct EventDetailView: View {
    let event: ClusterEvent

    var body: some View {
        List {
            Section("Event") {
                LabeledContent("Type", value: event.type)
                LabeledContent("Reason", value: event.reason.isEmpty ? "Unknown" : event.reason)
                LabeledContent("Object", value: event.object.isEmpty ? "Unknown" : event.object)
                LabeledContent("Namespace", value: event.namespace)
                LabeledContent("Source", value: event.source.isEmpty ? "Unknown" : event.source)
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
        VStack(alignment: .leading, spacing: 5) {
            Text(lease.name).font(.headline)
            Text(lease.namespace).font(.subheadline).foregroundStyle(.secondary)
            Text(lease.holder.isEmpty ? "No holder" : lease.holder)
                .font(.caption).foregroundStyle(.secondary).lineLimit(1)
        }
        .padding(.vertical, 3)
    }
}

private struct LeaseDetailView: View {
    let lease: ClusterLease

    var body: some View {
        List {
            Section("Lease") {
                LabeledContent("Namespace", value: lease.namespace)
                LabeledContent("Holder", value: lease.holder.isEmpty ? "None" : lease.holder)
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
        VStack(alignment: .leading, spacing: 5) {
            Text(forward.pod).font(.headline)
            Text(forward.namespace).font(.subheadline).foregroundStyle(.secondary)
            Text("localhost:\(forward.local) → \(forward.remote)")
                .font(.caption).foregroundStyle(.secondary).monospacedDigit()
        }
        .padding(.vertical, 3)
    }
}

private struct PortForwardDetailView: View {
    let forward: ClusterPortForward

    var body: some View {
        List {
            Section("Port-forward") {
                LabeledContent("Namespace", value: forward.namespace)
                LabeledContent("Pod", value: forward.pod)
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

private struct StatusText: View {
    let text: String
    let healthy: Bool

    var body: some View {
        Label(text, systemImage: healthy ? "checkmark.circle.fill" : "exclamationmark.triangle.fill")
            .font(.caption.weight(.medium))
            .foregroundStyle(healthy ? .green : .orange)
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
