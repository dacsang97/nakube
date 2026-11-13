import SwiftUI

struct ContextsHomeView: View {
    let connection: DesktopConnection
    let onConnect: (DesktopConnection, [ClusterContext]) -> Void
    @State private var contexts: [ClusterContext]?
    @State private var errorText: String?
    @State private var loading = false
    @State private var requestID = UUID()
    @State private var refreshID = UUID()
    @State private var showingConnection = false

    init(connection: DesktopConnection, initialContexts: [ClusterContext]?, onConnect: @escaping (DesktopConnection, [ClusterContext]) -> Void) {
        self.connection = connection
        self.onConnect = onConnect
        _contexts = State(initialValue: initialContexts)
    }

    var body: some View {
        NavigationStack {
            List {
                Section {
                    Label {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(connection.displayHost).textSelection(.enabled)
                            Text("Connected desktop").font(.caption).foregroundStyle(.secondary)
                        }
                    } icon: {
                        Image(systemName: "desktopcomputer")
                    }
                    Button { showingConnection = true } label: {
                        Label("Add or Change Connection", systemImage: "plus")
                    }
                } header: {
                    Text("Desktop")
                }
                if let errorText {
                    Section {
                        ErrorMessageView(title: "Couldn’t load contexts", message: errorText)
                        Button("Try Again") { refreshID = UUID() }
                    }
                }
                Section {
                    if loading && contexts == nil {
                        ForEach(0..<3, id: \.self) { _ in ContextRowSkeleton() }
                            .accessibilityElement(children: .ignore)
                            .accessibilityLabel("Loading contexts")
                    } else if let contexts, contexts.isEmpty {
                        ContentUnavailableView {
                            Label("No Contexts", systemImage: "square.stack.3d.up")
                        } description: {
                            Text("Add a Kubernetes context to your desktop kubeconfig, then refresh.")
                        }
                    } else if let contexts {
                        ForEach(contexts) { context in
                            NavigationLink(value: context) {
                                VStack(alignment: .leading, spacing: 6) {
                                    Text(context.name).font(.headline)
                                    if !context.cluster.isEmpty {
                                        Text(context.cluster).font(.subheadline).foregroundStyle(.secondary)
                                    }
                                    if context.current {
                                        Label("Current on desktop", systemImage: "checkmark.circle")
                                            .font(.caption).foregroundStyle(.secondary)
                                    }
                                }
                                .padding(.vertical, 4)
                            }
                            .accessibilityIdentifier("context.\(context.name)")
                        }
                    }
                } header: {
                    HStack {
                        Text("Kubernetes Contexts")
                        if loading && contexts != nil { ProgressView().accessibilityLabel("Refreshing contexts") }
                    }
                } footer: {
                    Text("Choose a context to view its cluster. This won’t switch the context on your desktop.")
                }
            }
            .navigationTitle("Contexts")
            .navigationDestination(for: ClusterContext.self) { context in
                ClusterOverviewView(connection: connection, context: context)
            }
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button { refreshID = UUID() } label: {
                        Label("Refresh Contexts", systemImage: "arrow.clockwise")
                    }
                    .disabled(loading)
                }
            }
            .refreshable { await refresh() }
            .task(id: refreshID) { await refresh() }
            .onDisappear { requestID = UUID() }
            .sheet(isPresented: $showingConnection) {
                NavigationStack {
                    ConnectionView(onConnect: { newConnection, contexts in
                        showingConnection = false
                        onConnect(newConnection, contexts)
                    })
                    .toolbar {
                        ToolbarItem(placement: .cancellationAction) {
                            Button("Cancel") { showingConnection = false }
                        }
                    }
                }
            }
        }
    }

    @MainActor
    private func refresh() async {
        let id = UUID()
        requestID = id
        loading = true
        errorText = nil
        defer { if requestID == id { loading = false } }
        do {
            let result = try await DesktopAPI(connection: connection).contexts()
            try Task.checkCancellation()
            guard requestID == id else { return }
            contexts = result
        } catch {
            guard !Task.isCancelled, requestID == id else { return }
            errorText = connectionMessage(error)
        }
    }
}
