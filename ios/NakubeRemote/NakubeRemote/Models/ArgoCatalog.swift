import Foundation

struct ArgoApplicationSet: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let generators: String
    let project: String
    let destination: String
    let conditions: [String]
    let createdAt: String

    var searchText: String {
        ([name, namespace, generators, project, destination] + conditions).joined(separator: " ")
    }
}

struct ArgoProject: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let description: String
    let sourceRepos: [String]
    let destinations: [String]
    let roles: Int
    let createdAt: String

    var searchText: String {
        ([name, namespace, description] + sourceRepos + destinations).joined(separator: " ")
    }
}

struct ArgoRepository: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let type: String
    let url: String
    let project: String

    var searchText: String { [name, namespace, type, url, project].joined(separator: " ") }
}

struct ArgoCluster: Decodable, ClusterListItem {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let server: String

    var searchText: String { [name, namespace, server].joined(separator: " ") }
}
