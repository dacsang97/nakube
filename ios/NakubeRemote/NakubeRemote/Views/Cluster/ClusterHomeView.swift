import SwiftUI

struct ClusterHomeView: View {
    let host: String
    let context: ClusterContext

    var body: some View {
        List {
            Section("Context") {
                LabeledContent("Name", value: context.name)
                if !context.cluster.isEmpty {
                    LabeledContent("Cluster", value: context.cluster)
                }
                LabeledContent("Desktop", value: host)
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
                    destination("Workloads", icon: "square.3.layers.3d")
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
        .navigationTitle("Cluster")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func destination(_ title: String, icon: String) -> some View {
        Label(title, systemImage: icon)
            .padding(.vertical, 4)
    }
}
