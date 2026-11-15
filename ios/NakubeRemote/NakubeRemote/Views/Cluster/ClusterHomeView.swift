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
            Section("Argo CD") {
                NavigationLink {
                    ArgoDashboardView(host: host, context: context)
                } label: {
                    destination("Dashboard", icon: "gauge.with.dots.needle.50percent")
                }
                NavigationLink {
                    ArgoApplicationsView(host: host, context: context)
                } label: {
                    destination("Applications", icon: "arrow.triangle.branch")
                }
                NavigationLink {
                    ArgoApplicationSetsView(host: host, context: context)
                } label: {
                    destination("ApplicationSets", icon: "square.stack.3d.up")
                }
                NavigationLink {
                    ArgoProjectsView(host: host, context: context)
                } label: {
                    destination("AppProjects", icon: "folder")
                }
                NavigationLink {
                    ArgoRepositoriesView(host: host, context: context)
                } label: {
                    destination("Repositories", icon: "externaldrive")
                }
                NavigationLink {
                    ArgoClustersView(host: host, context: context)
                } label: {
                    destination("Clusters", icon: "globe")
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
