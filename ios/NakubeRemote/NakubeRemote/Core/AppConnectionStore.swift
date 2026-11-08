import Foundation
import Observation

/// Owns the verified desktop connection independently of view lifetimes.
@MainActor
@Observable
final class AppConnectionStore {
    private(set) var host: String
    private(set) var initialContexts: [ClusterContext]?
    private(set) var connectionID = UUID()
    private(set) var migrating = false
    private(set) var migrationError: String?

    @ObservationIgnored private let defaults: UserDefaults
    @ObservationIgnored private var attemptedConnection = false

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        host = defaults.string(forKey: "successfulDesktopHost") ?? ""
    }

    func beginConnectionAttempt() {
        attemptedConnection = true
    }

    /// Call only after the desktop has returned a valid context catalog.
    func saveConnection(host newHost: String, contexts: [ClusterContext]) {
        attemptedConnection = true
        initialContexts = contexts
        connectionID = UUID()
        host = newHost
        defaults.set(newHost, forKey: "successfulDesktopHost")
        defaults.removeObject(forKey: "desktopHost")
    }

    func restoreLegacyConnection() async {
        // The old editable address is not proof of a successful connection.
        guard host.isEmpty, !attemptedConnection,
              let legacy = defaults.string(forKey: "desktopHost"),
              let candidate = tailscaleHost(from: legacy) else { return }
        migrating = true
        defer { migrating = false }
        do {
            let contexts = try await DesktopAPI(host: candidate).contexts()
            try Task.checkCancellation()
            guard host.isEmpty, !attemptedConnection else { return }
            saveConnection(host: candidate, contexts: contexts)
        } catch {
            guard !Task.isCancelled, !attemptedConnection else { return }
            migrationError = "Your previous address could not be verified. Scan the desktop QR code or enter its address to reconnect."
        }
    }
}
