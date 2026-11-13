import Foundation

struct DesktopAPI: Sendable {
    let connection: DesktopConnection
    private let session: URLSession

    init(connection: DesktopConnection, session: URLSession = .shared) {
        self.connection = connection
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
        var lastTransportError: Error?
        for endpoint in connection.endpoints {
            try Task.checkCancellation()
            do {
                return try await get(path: path, query: query, endpoint: endpoint)
            } catch let error as URLError {
                lastTransportError = error
            }
        }
        throw lastTransportError ?? RemoteError.invalidHost
    }

    private func get<Value: Decodable>(
        path: String, query: [URLQueryItem], endpoint: DesktopEndpoint
    ) async throws -> Value {
        var components = URLComponents()
        components.scheme = "http"
        components.host = endpoint.host
        components.port = endpoint.port
        components.path = path
        components.queryItems = query.isEmpty ? nil : query
        guard let url = components.url else { throw RemoteError.invalidHost }
        var request = URLRequest(url: url)
        request.timeoutInterval = 8
        request.cachePolicy = .reloadIgnoringLocalCacheData
        request.setValue("Bearer \(connection.token)", forHTTPHeaderField: "Authorization")
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
