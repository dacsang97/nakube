import Foundation

enum RemoteError: LocalizedError {
    case invalidHost
    case http(Int)
    case incompatibleDesktop
    case wrongContext

    var errorDescription: String? {
        switch self {
        case .invalidHost:
            return "Use the desktop’s full .ts.net MagicDNS address or paste its NaKube QR link. Numeric IP addresses are not supported by this app’s secure transport policy."
        case .http(let status):
            return "The desktop returned HTTP \(status). Keep NaKube running on your desktop and try again."
        case .incompatibleDesktop:
            return "The desktop returned an unsupported response. Update NaKube on your desktop, then try again."
        case .wrongContext:
            return "The desktop returned a different context. No cluster data was shown. Refresh or choose the context again."
        }
    }
}

func connectionMessage(_ error: Error) -> String {
    if let remote = error as? RemoteError { return remote.localizedDescription }
    if let urlError = error as? URLError {
        switch urlError.code {
        case .timedOut:
            return "The desktop did not respond. Check that Tailscale is connected on both devices and NaKube is running, then try again."
        case .notConnectedToInternet, .cannotFindHost, .cannotConnectToHost, .networkConnectionLost, .dnsLookupFailed:
            return "Cannot reach your desktop. Connect both devices to the same Tailscale network and keep NaKube running on your desktop."
        default:
            break
        }
    }
    return error.localizedDescription
}
