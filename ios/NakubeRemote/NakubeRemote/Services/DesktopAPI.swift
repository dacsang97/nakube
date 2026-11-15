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
            path: "/v1/cluster/overview", query: contextQuery(context)
        )
        guard result.context == context else { throw RemoteError.wrongContext }
        return result
    }

    func nodes(context: String) async throws -> [ClusterNode] {
        try await contextList(path: "/v1/cluster/nodes", context: context)
    }

    func namespaces(context: String) async throws -> [ClusterNamespace] {
        try await contextList(path: "/v1/cluster/namespaces", context: context)
    }

    func events(context: String) async throws -> [ClusterEvent] {
        try await contextList(path: "/v1/cluster/events", context: context)
    }

    func leases(context: String) async throws -> [ClusterLease] {
        try await contextList(path: "/v1/cluster/leases", context: context)
    }

    func portForwards() async throws -> [ClusterPortForward] {
        try await get(path: "/v1/port-forwards")
    }

    func workloads(context: String, resource: WorkloadResource? = nil) async throws -> [Workload] {
        let suffix = resource.map { "/\($0.rawValue)" } ?? ""
        return try await contextList(path: "/v1/workloads\(suffix)", context: context)
    }

    func streamLogs(
        context: String,
        namespace: String,
        pod: String,
        container: String,
        onLine: @escaping @MainActor @Sendable (String) -> Void
    ) async throws {
        let query = contextQuery(context) + [URLQueryItem(name: "container", value: container)]
        var request = try makeRequest(
            path: "/v1/workloads/pods/\(namespace)/\(pod)/logs",
            query: query
        )
        request.timeoutInterval = 24 * 60 * 60
        let (bytes, response) = try await session.bytes(for: request)
        try validate(response)
        for try await line in bytes.lines {
            try Task.checkCancellation()
            await onLine(line)
        }
    }

    func restartPod(context: String, namespace: String, name: String) async throws {
        try await mutatePod(action: "restart", context: context, namespace: namespace, name: name)
    }

    func deletePod(context: String, namespace: String, name: String) async throws {
        try await mutatePod(action: "delete", context: context, namespace: namespace, name: name)
    }

    private func mutatePod(action: String, context: String, namespace: String, name: String) async throws {
        let result: MutationResponse = try await send(
            method: "POST",
            path: "/v1/workloads/pods/\(namespace)/\(name)/\(action)",
            query: contextQuery(context)
        )
        guard result.ok else { throw RemoteError.incompatibleDesktop }
    }

    private struct MutationResponse: Decodable {
        let ok: Bool
    }

    private func contextList<Value: Decodable>(path: String, context: String) async throws -> Value {
        try await get(path: path, query: contextQuery(context))
    }

    private func contextQuery(_ context: String) -> [URLQueryItem] {
        [URLQueryItem(name: "context", value: context)]
    }

    private func get<Value: Decodable>(path: String, query: [URLQueryItem] = []) async throws -> Value {
        try await send(method: "GET", path: path, query: query)
    }

    private func send<Value: Decodable>(
        method: String,
        path: String,
        query: [URLQueryItem] = []
    ) async throws -> Value {
        try Task.checkCancellation()
        var request = try makeRequest(path: path, query: query)
        request.httpMethod = method
        let (data, response) = try await session.data(for: request)
        try Task.checkCancellation()
        try validate(response)
        do {
            return try JSONDecoder().decode(Value.self, from: data)
        } catch {
            throw RemoteError.incompatibleDesktop
        }
    }

    private func makeRequest(path: String, query: [URLQueryItem] = []) throws -> URLRequest {
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
        return request
    }

    private func validate(_ response: URLResponse) throws {
        guard let response = response as? HTTPURLResponse else { throw RemoteError.incompatibleDesktop }
        guard (200..<300).contains(response.statusCode) else { throw RemoteError.http(response.statusCode) }
    }
}
