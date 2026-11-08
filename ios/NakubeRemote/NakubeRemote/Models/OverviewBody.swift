import Foundation

struct OverviewBody: Decodable, Sendable {
    let context: String
    let nodes: String
    let pods: String
    let warnings: String
    let cpu: String
    let memory: String
    let charts: OverviewCharts?
}
