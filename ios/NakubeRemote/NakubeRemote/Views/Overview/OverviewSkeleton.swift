import SwiftUI

struct OverviewSkeleton: View {
    var body: some View {
        Section("Resource Usage") {
            VStack(alignment: .leading, spacing: 20) {
                HStack(spacing: 24) {
                    resourceSummary
                    resourceSummary
                }
                VStack(spacing: 30) {
                    ForEach(0..<4, id: \.self) { _ in
                        SkeletonView(height: 1, cornerRadius: 0)
                    }
                }
                .frame(height: 180)
            }
            .padding(.vertical, 8)
        }
        Section("Cluster Health") {
            healthSummary
            healthSummary
        }
        Section("Warnings") {
            SkeletonView(width: 160, height: 18).padding(.vertical, 8)
        }
    }

    private var resourceSummary: some View {
        VStack(alignment: .leading, spacing: 10) {
            SkeletonView(width: 70, height: 14)
            SkeletonView(height: 22)
            SkeletonView(width: 100, height: 12)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var healthSummary: some View {
        VStack(alignment: .leading, spacing: 16) {
            SkeletonView(width: 90, height: 18)
            HStack(spacing: 24) {
                Circle().stroke(.quaternary, lineWidth: 12)
                    .frame(width: 88, height: 88).padding(6)
                VStack(spacing: 16) {
                    SkeletonView(height: 14)
                    SkeletonView(height: 14)
                }
            }
        }
        .padding(.vertical, 8)
        .accessibilityHidden(true)
    }
}
