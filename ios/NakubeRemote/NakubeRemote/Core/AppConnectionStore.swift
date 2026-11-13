import Foundation
import Observation

/// Owns the verified, authenticated desktop connection independently of view lifetimes.
@MainActor
@Observable
final class AppConnectionStore {
    private(set) var connection: DesktopConnection?
    private(set) var initialContexts: [ClusterContext]?
    private(set) var connectionID = UUID()
    private(set) var migrating = false
    private(set) var migrationError: String?

    @ObservationIgnored private let defaults: UserDefaults
    @ObservationIgnored private let credentials: PairingCredentialStore
    @ObservationIgnored private var attemptedConnection = false

    init(defaults: UserDefaults = .standard, credentials: PairingCredentialStore = PairingCredentialStore()) {
        self.defaults = defaults
        self.credentials = credentials
        connection = credentials.load()
    }

    func beginConnectionAttempt() {
        attemptedConnection = true
    }

    /// Call only after the desktop has accepted the token and returned a valid catalog.
    func saveConnection(_ newConnection: DesktopConnection, contexts: [ClusterContext]) {
        attemptedConnection = true
        initialContexts = contexts
        connectionID = UUID()
        connection = newConnection
        migrationError = nil
        if !credentials.save(newConnection) {
            migrationError = "The pairing works now but could not be saved securely. Scan again after restarting the app."
        }
        defaults.removeObject(forKey: "successfulDesktopHost")
        defaults.removeObject(forKey: "desktopHost")
    }

    func restoreLegacyConnection() async {
        guard connection == nil, !attemptedConnection,
              defaults.string(forKey: "successfulDesktopHost") != nil
                || defaults.string(forKey: "desktopHost") != nil else { return }
        migrationError = "Desktop pairing is now authenticated. Scan the new desktop QR code once to reconnect; your old saved address was not sent over the network."
    }
}
