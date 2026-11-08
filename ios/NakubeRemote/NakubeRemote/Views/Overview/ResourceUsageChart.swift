import Charts
import SwiftUI

struct ResourceUsageChart: View {
    let overview: OverviewBody
    let charts: OverviewCharts

    private struct Reading: Identifiable {
        let name: String
        let percentage: Double
        let color: Color
        var id: String { name }
    }

    private var readings: [Reading] {
        var result: [Reading] = []
        if let percentage = charts.cpu.percentage {
            result.append(Reading(name: "CPU", percentage: percentage, color: .blue))
        }
        if let percentage = charts.memory.percentage {
            result.append(Reading(name: "Memory", percentage: percentage, color: .purple))
        }
        return result
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: 24) { summaries }
                VStack(alignment: .leading, spacing: 20) { summaries }
            }
            let readings = readings
            if readings.isEmpty {
                Label("Utilization unavailable. Metrics must cover every node.", systemImage: "chart.bar.xaxis")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, minHeight: 120, alignment: .center)
            } else {
                Chart {
                    ForEach(readings) { reading in
                        BarMark(
                            x: .value("Resource", reading.name),
                            y: .value("Allocatable used (%)", reading.percentage),
                            width: .ratio(0.4)
                        )
                        .foregroundStyle(reading.color.gradient)
                        .cornerRadius(6)
                        .accessibilityLabel(reading.name)
                        .accessibilityValue("\(reading.percentage.formatted(.number.precision(.fractionLength(1)))) percent of allocatable capacity")
                    }
                    RuleMark(y: .value("Allocatable capacity", 100))
                        .lineStyle(StrokeStyle(lineWidth: 1, dash: [4, 4]))
                        .foregroundStyle(.secondary)
                }
                .chartYScale(domain: 0...max(110, (readings.lazy.map(\.percentage).max() ?? 100) * 1.1))
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine()
                        AxisValueLabel {
                            if let percent = value.as(Double.self) {
                                Text("\(percent.formatted(.number.precision(.fractionLength(0))))%")
                            }
                        }
                    }
                }
                .frame(height: 180)
            }
        }
        .padding(.vertical, 8)
    }

    @ViewBuilder
    private var summaries: some View {
        summary("CPU", value: overview.cpu, reading: charts.cpu, color: .blue)
        summary("Memory", value: overview.memory, reading: charts.memory, color: .purple)
    }

    private func summary(_ title: String, value: String, reading: ResourceReading, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Label(title, systemImage: title == "CPU" ? "cpu" : "memorychip")
                .font(.subheadline.weight(.medium))
                .foregroundStyle(color)
            Text(value).font(.title3.weight(.semibold)).monospacedDigit()
            if let percentage = reading.percentage {
                Text("\(percentage.formatted(.number.precision(.fractionLength(1))))% of allocatable")
                    .font(.caption).foregroundStyle(.secondary)
            } else {
                Text("Utilization unavailable").font(.caption).foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
