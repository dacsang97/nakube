import Foundation

struct DesktopEndpoint: Codable, Hashable, Sendable {
    let network: String
    let host: String
    let port: Int

    var label: String { network == "tailscale" ? "Tailscale" : "LAN" }
}

struct DesktopConnection: Codable, Hashable, Sendable {
    let endpoints: [DesktopEndpoint]
    let token: String

    var displayHost: String { endpoints.first?.host ?? "" }
}

func desktopConnection(from input: String) -> DesktopConnection? {
    let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let components = URLComponents(string: trimmed),
          components.scheme?.lowercased() == "nakube",
          components.host?.lowercased() == "connect",
          let items = components.queryItems,
          items.first(where: { $0.name == "v" })?.value == "2",
          let token = items.first(where: { $0.name == "token" })?.value,
          isPairingToken(token) else { return nil }

    let portText = items.first(where: { $0.name == "port" })?.value ?? "47623"
    guard let port = Int(portText), port == 47623 else { return nil }

    var endpoints: [DesktopEndpoint] = []
    var seen = Set<String>()
    for item in items where item.name == "lan" || item.name == "tailscale" {
        guard let raw = item.value else { continue }
        let host = normalizedHost(raw)
        let valid = item.name == "lan" ? isPrivateIPv4(host) : isTailscaleHost(host)
        guard valid, seen.insert("\(item.name):\(host)").inserted else { continue }
        endpoints.append(DesktopEndpoint(network: item.name, host: host, port: port))
    }
    guard !endpoints.isEmpty else { return nil }
    return DesktopConnection(endpoints: endpoints, token: token)
}

private func isPairingToken(_ token: String) -> Bool {
    (32...128).contains(token.utf8.count) && token.utf8.allSatisfy {
        (65...90).contains($0) || (97...122).contains($0) || (48...57).contains($0) || $0 == 45 || $0 == 95
    }
}

private func normalizedHost(_ raw: String) -> String {
    var host = raw.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    if host.hasSuffix(".") { host.removeLast() }
    return host
}

private func isPrivateIPv4(_ host: String) -> Bool {
    guard let parts = ipv4Octets(host) else { return false }
    return parts[0] == 10
        || (parts[0] == 172 && (16...31).contains(parts[1]))
        || (parts[0] == 192 && parts[1] == 168)
}

private func isTailscaleHost(_ host: String) -> Bool {
    if isValidDNSName(host), host.hasSuffix(".ts.net") { return true }
    guard let parts = ipv4Octets(host) else { return false }
    return parts[0] == 100 && (64...127).contains(parts[1])
}

private func ipv4Octets(_ host: String) -> [UInt8]? {
    let fields = host.split(separator: ".", omittingEmptySubsequences: false)
    guard fields.count == 4 else { return nil }
    let octets = fields.compactMap { UInt8(String($0)) }
    return octets.count == 4 ? octets : nil
}

private func isValidDNSName(_ host: String) -> Bool {
    guard !host.isEmpty, host.utf8.count <= 253 else { return false }
    let labels = host.split(separator: ".", omittingEmptySubsequences: false)
    return labels.count >= 3 && labels.allSatisfy { label in
        !label.isEmpty && label.utf8.count <= 63 && label.first != "-" && label.last != "-"
            && label.utf8.allSatisfy { (97...122).contains($0) || (48...57).contains($0) || $0 == 45 }
    }
}
