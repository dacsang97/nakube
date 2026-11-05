import SwiftUI

struct PodRow: Decodable, Identifiable {
    var id: String { "\(namespace)/\(name)" }
    let namespace: String
    let name: String
    let phase: String
    let owned: Bool
}

struct OverviewBody: Decodable {
    let context: String
    let nodes: String
    let pods: String
    let warnings: String
    let cpu: String
    let memory: String
}

struct ArgoRow: Decodable, Identifiable {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let sync: String
    let health: String
}

struct ContentView: View {
    @AppStorage("desktopHost") private var host = ""
    @State private var overview: OverviewBody?
    @State private var pods: [PodRow] = []
    @State private var apps: [ArgoRow] = []
    @State private var errorText = ""
    @State private var pending: PendingAction?

    var body: some View {
        NavigationStack {
            List {
                Section("Desktop") {
                    TextField("MagicDNS name", text: $host)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("Refresh") { Task { await refresh() } }
                }
                if !errorText.isEmpty {
                    Section { Text(errorText).foregroundStyle(.red) }
                }
                if let overview {
                    Section("Overview") {
                        LabeledContent("Context", value: overview.context)
                        LabeledContent("Nodes", value: overview.nodes)
                        LabeledContent("Pods", value: overview.pods)
                        LabeledContent("Warnings", value: overview.warnings)
                        LabeledContent("CPU", value: overview.cpu)
                        LabeledContent("Memory", value: overview.memory)
                    }
                }
                Section("Pods") {
                    ForEach(pods) { pod in
                        VStack(alignment: .leading) {
                            Text(pod.name)
                            Text("\(pod.namespace) · \(pod.phase)").font(.caption).foregroundStyle(.secondary)
                        }
                        .swipeActions {
                            Button("Delete", role: .destructive) {
                                pending = PendingAction(pod: pod, restart: false)
                            }
                            Button("Restart") {
                                pending = PendingAction(pod: pod, restart: true)
                            }
                        }
                    }
                }
                Section("Argo") {
                    ForEach(apps) { app in
                        VStack(alignment: .leading) {
                            Text(app.name)
                            Text("\(app.namespace) · \(app.sync) · \(app.health)").font(.caption).foregroundStyle(.secondary)
                        }
                    }
                }
            }
            .navigationTitle("Nakube")
            .confirmationDialog(pending?.title ?? "", isPresented: pendingPresented) {
                Button(pending?.verb ?? "Confirm", role: .destructive) {
                    Task { await confirmPending() }
                }
                Button("Cancel", role: .cancel) { pending = nil }
            } message: {
                Text(pending?.message ?? "")
            }
        }
    }

    private var pendingPresented: Binding<Bool> {
        Binding(get: { pending != nil }, set: { if !$0 { pending = nil } })
    }

    private func refresh() async {
        errorText = ""
        guard let base = baseURL else {
            errorText = "Enter the desktop MagicDNS name."
            return
        }
        do {
            overview = try await get(base.appending(path: "v1/overview"))
            pods = try await get(base.appending(path: "v1/pods"))
            apps = try await get(base.appending(path: "v1/argo/applications"))
        } catch {
            errorText = error.localizedDescription
        }
    }

    private func confirmPending() async {
        guard let action = pending, let base = baseURL else { return }
        pending = nil
        let verb = action.restart ? "restart" : "delete"
        if action.restart && !action.pod.owned {
            errorText = "This pod has no controller. Use delete."
            return
        }
        var request = URLRequest(url: base.appending(path: "v1/pods/\(action.pod.namespace)/\(action.pod.name)/\(verb)"))
        request.httpMethod = "POST"
        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            if let http = response as? HTTPURLResponse, http.statusCode >= 300 {
                errorText = "The desktop refused \(verb)."
            }
            await refresh()
        } catch {
            errorText = error.localizedDescription
        }
    }

    private var baseURL: URL? {
        let trimmed = host.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return nil }
        return URL(string: "http://\(trimmed):47623")
    }

    private func get<T: Decodable>(_ url: URL) async throws -> T {
        let (data, response) = try await URLSession.shared.data(from: url)
        if let http = response as? HTTPURLResponse, http.statusCode >= 300 {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode(T.self, from: data)
    }
}

private struct PendingAction {
    let pod: PodRow
    let restart: Bool
    var verb: String { restart ? "Restart" : "Delete" }
    var title: String { "\(verb) \(pod.name)?" }
    var message: String {
        restart
            ? "Restart deletes the owned pod so its controller can recreate it."
            : "Delete \(pod.namespace)/\(pod.name)."
    }
}
