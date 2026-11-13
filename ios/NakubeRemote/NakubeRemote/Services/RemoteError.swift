import Foundation

enum RemoteError: LocalizedError {
    case invalidHost
    case http(Int)
    case incompatibleDesktop
    case wrongContext

    var errorDescription: String? {
        switch self {
        case .invalidHost:
            return "Scan the desktop QR code or paste its complete NaKube pairing link. A LAN or Tailscale address without its pairing credential is not enough."
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
            return "The desktop did not respond. Use the same LAN or connect Tailscale on both devices, then try again."
        case .notConnectedToInternet, .cannotFindHost, .cannotConnectToHost, .networkConnectionLost, .dnsLookupFailed:
            return "Cannot reach your desktop over the paired LAN or Tailscale addresses. Keep NaKube running on your desktop and try again."
        default:
            break
        }
    }
    return error.localizedDescription
}
