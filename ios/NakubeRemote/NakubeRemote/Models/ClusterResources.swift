import Foundation

protocol ClusterListItem: Identifiable, Sendable {
    var searchText: String { get }
}

struct ClusterNode: Decodable, ClusterListItem {
    var id: String { name }
    let name: String
    let ready: Bool
    let roles: [String]
    let version: String
    let conditions: [String]
    let taints: [String]
    let cpu: ResourceReading
    let memory: ResourceReading
    let createdAt: String

    var searchText: String {
        ([name, version] + roles + conditions + taints).joined(separator: " ")
    }
}

struct ClusterNamespace: Decodable, ClusterListItem {
    var id: String { name }
    let name: String
    let phase: String
    let labels: [String: String]
    let finalizers: [String]
    let createdAt: String

    var searchText: String {
        ([name, phase] + labels.map { "\($0.key)=\($0.value)" } + finalizers).joined(separator: " ")
    }
}

struct ClusterEvent: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let namespace: String
    let name: String
    let type: String
    let reason: String
    let object: String
    let message: String
    let source: String
    let count: Int
    let firstSeen: String
    let lastSeen: String

    var searchText: String {
        [namespace, name, type, reason, object, message, source].joined(separator: " ")
    }
}

struct ClusterLease: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let namespace: String
    let name: String
    let holder: String
    let renewTime: String?
    let durationSeconds: Int?
    let transitions: Int?
    let createdAt: String

    var searchText: String {
        [namespace, name, holder].joined(separator: " ")
    }
}

struct ClusterPortForward: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(pod):\(local):\(remote)" }
    let namespace: String
    let pod: String
    let local: UInt16
    let remote: UInt16

    var searchText: String {
        [namespace, pod, String(local), String(remote)].joined(separator: " ")
    }
}
