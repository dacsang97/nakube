import Foundation

struct Workload: Decodable, ClusterListItem {
    var id: String { "\(resource)/\(namespace)/\(name)" }
    let resource: String
    let kind: String
    let namespace: String
    let name: String
    let status: String
    let ready: Int
    let desired: Int
    let createdAt: String
    let containers: [WorkloadContainer]
    let facts: [WorkloadFact]
    let owned: Bool

    var searchText: String {
        ([resource, kind, namespace, name, status] + containers.flatMap { [$0.name, $0.image] }).joined(separator: " ")
    }

    var readyText: String {
        desired > 0 ? "\(ready)/\(desired)" : "—"
    }

    var healthy: Bool {
        status == "Healthy" || status == "Succeeded" || status == "Scheduled"
    }
}

struct WorkloadContainer: Decodable, Identifiable, Sendable {
    var id: String { name }
    let name: String
    let image: String
    let ready: Bool
    let restarts: Int
}

struct WorkloadFact: Decodable, Identifiable, Sendable {
    var id: String { label }
    let label: String
    let value: String
}

enum WorkloadResource: String, CaseIterable, Identifiable, Sendable {
    case pods
    case deployments
    case replicasets
    case daemonsets
    case statefulsets
    case jobs
    case cronjobs

    var id: String { rawValue }

    var title: String {
        switch self {
        case .pods: "Pods"
        case .deployments: "Deployments"
        case .replicasets: "ReplicaSets"
        case .daemonsets: "DaemonSets"
        case .statefulsets: "StatefulSets"
        case .jobs: "Jobs"
        case .cronjobs: "CronJobs"
        }
    }

    var icon: String {
        switch self {
        case .pods: "shippingbox"
        case .deployments: "arrow.up.forward.square"
        case .replicasets: "square.on.square"
        case .daemonsets: "cpu"
        case .statefulsets: "externaldrive"
        case .jobs: "play.circle"
        case .cronjobs: "clock.arrow.circlepath"
        }
    }
}
