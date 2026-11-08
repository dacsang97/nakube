import SwiftUI

/// A neutral loading shape, never a fabricated measurement.
struct SkeletonView: View {
    var width: CGFloat?
    var height: CGFloat = 14
    var cornerRadius: CGFloat = 6
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var pulsing = false

    var body: some View {
        RoundedRectangle(cornerRadius: cornerRadius)
            .fill(.quaternary)
            .frame(width: width, height: height)
            .opacity(reduceMotion || !pulsing ? 1 : 0.5)
            .animation(reduceMotion ? nil : .easeInOut(duration: 1.2).repeatForever(autoreverses: true), value: pulsing)
            .task(id: reduceMotion) { pulsing = !reduceMotion }
            .accessibilityHidden(true)
    }
}

struct ContextRowSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            SkeletonView(height: 18)
            SkeletonView(width: 180, height: 13)
            SkeletonView(width: 110, height: 11)
        }
        .padding(.vertical, 8)
        .accessibilityHidden(true)
    }
}
