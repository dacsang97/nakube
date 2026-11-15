import Foundation

enum RemoteDate {
    private static let plain = Date.ISO8601FormatStyle()
    private static let fractional = Date.ISO8601FormatStyle(includingFractionalSeconds: true)

    /// `abbreviated` gives "5 min. ago" for trailing row metadata where width is tight.
    static func relative(_ raw: String?, abbreviated: Bool = false) -> String {
        guard let raw, !raw.hasPrefix("0001-"), let date = parse(raw) else {
            return "Unknown"
        }
        return date.formatted(.relative(presentation: .named, unitsStyle: abbreviated ? .abbreviated : .wide))
    }

    private static func parse(_ raw: String) -> Date? {
        if let date = try? Date(raw, strategy: fractional) {
            return date
        }
        return try? Date(raw, strategy: plain)
    }
}
