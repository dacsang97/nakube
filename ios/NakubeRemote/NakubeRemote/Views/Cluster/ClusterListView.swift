import SwiftUI

struct ClusterListView<Item: ClusterListItem, RowContent: View, DetailContent: View>: View {
    let title: String
    let emptyTitle: String
    let emptyMessage: String
    let emptyIcon: String
    let load: () async throws -> [Item]
    let rowContent: (Item) -> RowContent
    /// Receives a reload action so a detail screen that changes the item can refresh this list.
    let detailContent: (Item, @escaping () -> Void) -> DetailContent

    init(
        title: String,
        emptyTitle: String,
        emptyMessage: String,
        emptyIcon: String,
        load: @escaping () async throws -> [Item],
        @ViewBuilder rowContent: @escaping (Item) -> RowContent,
        @ViewBuilder detailContent: @escaping (Item, @escaping () -> Void) -> DetailContent
    ) {
        self.title = title
        self.emptyTitle = emptyTitle
        self.emptyMessage = emptyMessage
        self.emptyIcon = emptyIcon
        self.load = load
        self.rowContent = rowContent
        self.detailContent = detailContent
    }

    @State private var items: [Item]?
    @State private var query = ""
    @State private var errorText: String?
    @State private var loading = false
    @State private var refreshID = UUID()
    @State private var requestID = UUID()

    private var filteredItems: [Item] {
        guard let items else { return [] }
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return items }
        return items.filter { $0.searchText.localizedStandardContains(trimmed) }
    }

    var body: some View {
        List {
            if let errorText {
                Section {
                    ErrorMessageView(title: "Couldn’t load \(title.lowercased())", message: errorText)
                    Button("Try Again") { refreshID = UUID() }
                }
            }
            if loading && items == nil {
                Section {
                    ForEach(0..<5, id: \.self) { _ in ContextRowSkeleton() }
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel("Loading \(title.lowercased())")
            } else if let items {
                let visibleItems = filteredItems
                if items.isEmpty {
                    ContentUnavailableView {
                        Label(emptyTitle, systemImage: emptyIcon)
                    } description: {
                        Text(emptyMessage)
                    }
                } else if visibleItems.isEmpty {
                    ContentUnavailableView.search(text: query)
                } else {
                    ForEach(visibleItems) { item in
                        NavigationLink {
                            detailContent(item) { refreshID = UUID() }
                        } label: {
                            rowContent(item)
                        }
                    }
                }
            }
        }
        .navigationTitle(title)
        .navigationBarTitleDisplayMode(.inline)
        .searchable(text: $query, prompt: "Search \(title.lowercased())")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button { refreshID = UUID() } label: {
                    Label("Refresh \(title)", systemImage: "arrow.clockwise")
                }
                .disabled(loading)
            }
        }
        .refreshable { await refresh() }
        .task(id: refreshID) { await refresh() }
        .onDisappear { requestID = UUID() }
    }

    @MainActor
    private func refresh() async {
        let id = UUID()
        requestID = id
        loading = true
        errorText = nil
        defer { if requestID == id { loading = false } }
        do {
            let result = try await load()
            try Task.checkCancellation()
            guard requestID == id else { return }
            items = result
        } catch {
            guard !Task.isCancelled, requestID == id else { return }
            errorText = connectionMessage(error)
        }
    }
}
