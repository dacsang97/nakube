import SwiftUI

/// Row status for long Kubernetes lists. Healthy rows show only a quiet check so the few
/// rows that need attention carry the words and the colour, and stand out when scanning.
struct StatusBadge: View {
    enum Tone { case ok, pending, warning, error }

    let text: String
    let tone: Tone

    var body: some View {
        Group {
            if tone == .ok {
                Image(systemName: "checkmark.circle.fill")
            } else {
                HStack(spacing: 3) {
                    Image(systemName: icon)
                    Text(text)
                }
            }
        }
        .font(.caption.weight(.semibold))
        .foregroundStyle(color)
        .fixedSize()
        .layoutPriority(1)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(text)
    }

    private var icon: String {
        switch tone {
        case .ok: "checkmark.circle.fill"
        case .pending: "clock.fill"
        case .warning: "exclamationmark.triangle.fill"
        case .error: "xmark.octagon.fill"
        }
    }

    private var color: Color {
        switch tone {
        case .ok: .green
        case .pending: .orange
        case .warning: .orange
        case .error: .red
        }
    }
}

extension Workload {
    var statusTone: StatusBadge.Tone {
        switch status {
        case "Healthy", "Succeeded", "Scheduled": .ok
        case "Failed", "Unknown": .error
        default: .pending
        }
    }
}
