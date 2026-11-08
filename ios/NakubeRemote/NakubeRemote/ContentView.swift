import SwiftUI

struct ContentView: View {
    @State private var connection = AppConnectionStore()

    var body: some View {
        Group {
            if !connection.host.isEmpty {
                ContextsHomeView(
                    host: connection.host,
                    initialContexts: connection.initialContexts,
                    onConnect: connection.saveConnection
                )
                .id(connection.connectionID)
            } else {
                NavigationStack {
                    ConnectionView(
                        restoring: connection.migrating,
                        restorationError: connection.migrationError,
                        onAttempt: connection.beginConnectionAttempt,
                        onConnect: connection.saveConnection
                    )
                }
            }
        }
        .task { await connection.restoreLegacyConnection() }
    }
}
