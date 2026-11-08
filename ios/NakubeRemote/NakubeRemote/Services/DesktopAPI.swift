import Foundation

struct DesktopAPI: Sendable {
    let host: String
    private let session: URLSession

    init(host: String, session: URLSession = .shared) {
        self.host = host
        self.session = session
    }

    func contexts() async throws -> [ClusterContext] {
        try await get(path: "/v1/contexts")
    }

    func overview(context: String) async throws -> OverviewBody {
        let result: OverviewBody = try await get(
            path: "/v1/overview", query: [URLQueryItem(name: "context", value: context)]
        )
        guard result.context == context else { throw RemoteError.wrongContext }
        return result
    }

    private func get<Value: Decodable>(path: String, query: [URLQueryItem] = []) async throws -> Value {
        try Task.checkCancellation()
        var components = URLComponents()
        components.scheme = "http"
        components.host = host
        components.port = 47623
        components.path = path
        components.queryItems = query.isEmpty ? nil : query
        guard let url = components.url else { throw RemoteError.invalidHost }
        var request = URLRequest(url: url)
        request.timeoutInterval = 20
        request.cachePolicy = .reloadIgnoringLocalCacheData
        let (data, response) = try await session.data(for: request)
        try Task.checkCancellation()
        guard let response = response as? HTTPURLResponse else { throw RemoteError.incompatibleDesktop }
        guard (200..<300).contains(response.statusCode) else { throw RemoteError.http(response.statusCode) }
        do {
            return try JSONDecoder().decode(Value.self, from: data)
        } catch {
            throw RemoteError.incompatibleDesktop
        }
    }
}
