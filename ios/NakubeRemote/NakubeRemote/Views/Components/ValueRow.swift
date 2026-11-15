import SwiftUI

/// A label/value row that stays on one line when it fits and stacks when it doesn't.
/// `LabeledContent` alone squeezes long values (ARNs, node hostnames, images) into a
/// narrow right-aligned column that wraps awkwardly on iPhone.
struct ValueRow: View {
    let title: String
    let value: String

    init(_ title: String, value: String) {
        self.title = title
        self.value = value
    }

    var body: some View {
        ViewThatFits(in: .horizontal) {
            LabeledContent(title) {
                Text(value).lineLimit(1)
            }
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.subheadline).foregroundStyle(.secondary)
                Text(value).textSelection(.enabled)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, 2)
            .accessibilityElement(children: .combine)
        }
    }
}
