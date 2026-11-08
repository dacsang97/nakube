import Charts
import SwiftUI

struct ClusterHealthChart: View {
    let title: String
    let healthyLabel: String
    let otherLabel: String
    let healthy: Int
    let total: Int
    let color: Color

    var body: some View {
        let otherCount = max(0, total - healthy)

        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.headline)
                Spacer()
                Text("\(healthy)/\(total)").font(.headline).monospacedDigit()
            }
            if total > 0 {
                HStack(spacing: 24) {
                    Chart {
                        SectorMark(angle: .value(healthyLabel, healthy), innerRadius: .ratio(0.75), angularInset: 2)
                            .foregroundStyle(color)
                        SectorMark(angle: .value(otherLabel, otherCount), innerRadius: .ratio(0.75), angularInset: 2)
                            .foregroundStyle(Color.secondary.opacity(0.2))
                    }
                    .frame(width: 100, height: 100)
                    .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 12) {
                        legend(healthyLabel, count: healthy, color: color)
                        legend(otherLabel, count: otherCount, color: .secondary)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            } else {
                Text("No \(title.lowercased()) in this cluster.")
                    .font(.subheadline).foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, minHeight: 72, alignment: .leading)
            }
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(title): \(healthy) \(healthyLabel.lowercased()), \(otherCount) \(otherLabel.lowercased()), \(total) total")
    }

    private func legend(_ label: String, count: Int, color: Color) -> some View {
        HStack(spacing: 8) {
            Circle().fill(color).frame(width: 7, height: 7)
            Text(label).font(.subheadline).foregroundStyle(.secondary)
            Spacer(minLength: 8)
            Text(count, format: .number).font(.subheadline.weight(.semibold)).monospacedDigit()
        }
    }
}
