import SwiftUI

struct ClusterHomeView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        List {
            Section("Context") {
                ValueRow("Name", value: context.name)
                if !context.cluster.isEmpty && context.cluster != context.name {
                    ValueRow("Cluster", value: context.cluster)
                }
                ValueRow("Desktop", value: host)
            }
            Section("Cluster") {
                NavigationLink {
                    ClusterOverviewView(host: host, context: context)
                } label: {
                    destination("Overview", icon: "chart.bar.xaxis")
                }
                NavigationLink {
                    ClusterNodesView(host: host, context: context)
                } label: {
                    destination("Nodes", icon: "server.rack")
                }
                NavigationLink {
                    ClusterNamespacesView(host: host, context: context)
                } label: {
                    destination("Namespaces", icon: "square.stack.3d.up")
                }
                NavigationLink {
                    ClusterEventsView(host: host, context: context)
                } label: {
                    destination("Events", icon: "bolt.horizontal.circle")
                }
                NavigationLink {
                    ClusterLeasesView(host: host, context: context)
                } label: {
                    destination("Leases", icon: "timer")
                }
                NavigationLink {
                    ClusterPortForwardsView(host: host)
                } label: {
                    destination("Port-forwards", icon: "arrow.left.arrow.right")
                }
            }
            Section("Workloads") {
                NavigationLink {
                    WorkloadsHomeView(host: host, context: context)
                } label: {
                    destination("All Workloads", icon: "square.grid.2x2")
                }
                ForEach(WorkloadResource.allCases) { resource in
                    NavigationLink {
                        WorkloadResourceView(host: host, context: context, resource: resource)
                    } label: {
                        destination(resource.title, icon: resource.icon)
                    }
                }
            }
        }
        .accessibilityIdentifier("cluster.home")
        .navigationTitle(context.displayName)
        .navigationBarTitleDisplayMode(.inline)
    }

    private func destination(_ title: String, icon: String) -> some View {
        Label(title, systemImage: icon)
            .padding(.vertical, 4)
    }
}
