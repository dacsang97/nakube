import Foundation

enum RemoteDate {
    private static let plain = Date.ISO8601FormatStyle()
    private static let fractional = Date.ISO8601FormatStyle(includingFractionalSeconds: true)

    static func relative(_ raw: String?) -> String {
        guard let raw, !raw.hasPrefix("0001-"), let date = parse(raw) else {
            return "Unknown"
        }
        return date.formatted(.relative(presentation: .named))
    }

    private static func parse(_ raw: String) -> Date? {
        if let date = try? Date(raw, strategy: fractional) {
            return date
        }
        return try? Date(raw, strategy: plain)
    }
}
