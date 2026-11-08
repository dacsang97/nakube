import Foundation

func tailscaleHost(from input: String) -> String? {
    let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
    let candidate: String?
    if let components = URLComponents(string: trimmed), let scheme = components.scheme?.lowercased() {
        switch scheme {
        case "nakube":
            candidate = components.queryItems?.first(where: { $0.name == "host" })?.value
        case "http", "https":
            guard components.user == nil, components.password == nil,
                  components.port == nil || components.port == 47623 else { return nil }
            candidate = components.host
        default:
            return nil
        }
    } else {
        candidate = trimmed
    }
    guard var host = candidate?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() else { return nil }
    if host.hasSuffix(".") { host.removeLast() }
    guard host.hasSuffix(".ts.net"), host.utf8.count <= 253 else { return nil }
    let labels = host.split(separator: ".", omittingEmptySubsequences: false)
    guard labels.count >= 3, labels.allSatisfy({ label in
        !label.isEmpty && label.utf8.count <= 63 && label.first != "-" && label.last != "-"
            && label.utf8.allSatisfy { (97...122).contains($0) || (48...57).contains($0) || $0 == 45 }
    }) else { return nil }
    return host
}
