import Foundation

struct ClusterContext: Decodable, Hashable, Identifiable, Sendable {
    var id: String { name }
    let name: String
    let cluster: String
    let current: Bool
}

extension ClusterContext {
    /// Short, human name for lists and titles. Cloud kubeconfigs name contexts with full
    /// resource identifiers (EKS ARNs, GKE triples) that wrap across several lines on a phone.
    var displayName: String { Self.parse(name).name }

    /// Where the cluster lives, shown under the display name; nil when it would repeat it.
    var displayDetail: String? {
        if let provider = Self.parse(name).provider { return provider }
        guard !cluster.isEmpty, cluster != name else { return nil }
        return cluster
    }

    private static func parse(_ raw: String) -> (name: String, provider: String?) {
        // arn:aws:eks:<region>:<account>:cluster/<name>
        let arn = raw.split(separator: ":", maxSplits: 5, omittingEmptySubsequences: false)
        if arn.count == 6, arn[0] == "arn", arn[2] == "eks", arn[5].hasPrefix("cluster/") {
            return (String(arn[5].dropFirst("cluster/".count)), "EKS · \(arn[3])")
        }
        // gke_<project>_<location>_<name>
        let gke = raw.split(separator: "_", maxSplits: 3, omittingEmptySubsequences: false)
        if gke.count == 4, gke[0] == "gke", !gke[3].isEmpty {
            return (String(gke[3]), "GKE · \(gke[2])")
        }
        return (raw, nil)
    }
}
