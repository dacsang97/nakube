import Foundation

struct ArgoApplication: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }

    let name: String
    let namespace: String
    let project: String
    let sync: String
    let health: String
    let healthMessage: String
    let revision: String
    let targetRevision: String
    let destination: String
    let policy: String
    let source: String
    let sources: [ArgoSource]
    let resources: [ArgoResource]
    let history: [ArgoHistory]
    let conditions: [String]
    let operationPhase: String
    let operationMessage: String
    let operationBy: String
    let images: [String]
    let sourceType: String
    let createdAt: String

    var searchText: String {
        let sourceText = sources.flatMap { [$0.repoURL, $0.path, $0.chart, $0.targetRevision, $0.ref] }
        let resourceText = resources.flatMap { [$0.kind, $0.namespace, $0.name, $0.sync, $0.health] }
        return ([name, namespace, project, sync, health, revision, targetRevision, destination, policy, source, sourceType]
                + sourceText + resourceText + conditions + images).joined(separator: " ")
    }
}

struct ArgoSource: Decodable, Sendable {
    let repoURL: String
    let path: String
    let chart: String
    let targetRevision: String
    let ref: String
}

struct ArgoResource: Decodable, Sendable, Identifiable {
    var id: String { "\(kind)/\(namespace)/\(name)" }

    let kind: String
    let namespace: String
    let name: String
    let sync: String
    let health: String
    let hook: Bool
}

struct ArgoHistory: Decodable, Sendable, Identifiable {
    var id: Int64 { historyID }

    private let historyID: Int64
    let revision: String
    let by: String

    private enum CodingKeys: String, CodingKey {
        case historyID = "id"
        case revision
        case by
    }
}

extension ArgoApplication {
    var healthTone: StatusBadge.Tone {
        switch health.lowercased() {
        case "healthy": .ok
        case "degraded", "missing": .error
        case "progressing", "suspended": .pending
        default: .warning
        }
    }

    var syncTone: StatusBadge.Tone {
        switch sync.lowercased() {
        case "synced": .ok
        case "outofsync": .warning
        default: .pending
        }
    }
}
