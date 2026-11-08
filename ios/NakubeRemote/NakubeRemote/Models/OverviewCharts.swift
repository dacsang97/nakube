import Foundation

struct OverviewCharts: Decodable, Sendable {
    let nodesReady: Int
    let nodesTotal: Int
    let podsRunning: Int
    let podsTotal: Int
    let cpu: ResourceReading
    let memory: ResourceReading
}

struct ResourceReading: Decodable, Sendable {
    let used: Int64?
    let allocatable: Int64

    var percentage: Double? {
        guard let used, used >= 0, allocatable > 0 else { return nil }
        return Double(used) / Double(allocatable) * 100
    }
}
