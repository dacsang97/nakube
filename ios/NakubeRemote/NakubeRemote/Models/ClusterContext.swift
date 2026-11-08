import Foundation

struct ClusterContext: Decodable, Hashable, Identifiable, Sendable {
    var id: String { name }
    let name: String
    let cluster: String
    let current: Bool
}
