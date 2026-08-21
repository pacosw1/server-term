import SwiftUI
import ServtermKit

struct ServerDetailView: View {
    @Environment(AppModel.self) private var model
    let server: ServerEntry

    private var reading: Reading<Sample>? { model.servers[server.id] }
    private var sample: Sample? { reading?.value }

    var body: some View {
        List {
            if let error = reading?.error {
                ErrorBanner(message: error)
            }
            if let sample {
                machineSection(sample)
                cpuSection(sample)
                memorySection(sample)
                storageSection(sample)
                networkSection(sample)
                if !sample.accelerators.isEmpty { acceleratorSection(sample) }
                processSection(sample)
            } else {
                Section {
                    Text("The app has no reading of this server yet.")
                        .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle(server.name)
        .navigationBarTitleDisplayMode(.inline)
        .pollEvery(seconds: 3) {
            await model.refresh(server: server)
        }
    }

    private func machineSection(_ sample: Sample) -> some View {
        Section("Machine") {
            MetricRow(label: "Host", value: sample.hostname.isEmpty ? Format.unknown : sample.hostname)
            MetricRow(label: "System", value: sample.os.isEmpty ? Format.unknown : sample.os)
            MetricRow(label: "Kernel", value: sample.kernel.isEmpty ? Format.unknown : sample.kernel)
            MetricRow(label: "Uptime", value: Format.duration(seconds: sample.uptimeSeconds))
            MetricRow(label: "State", value: sample.online ? "online" : "offline")
            AgeNote(fetchedAt: reading?.fetchedAt, isStale: reading?.error != nil)
        }
    }

    private func cpuSection(_ sample: Sample) -> some View {
        Section("CPU") {
            PercentBar(label: "Use", percent: sample.cpuPercent, detail: "\(sample.cores) cores")
            MetricRow(
                label: "Load",
                value: String(format: "%.2f  %.2f  %.2f", sample.load1, sample.load5, sample.load15))
            if let power = sample.power {
                MetricRow(label: "Power", value: String(format: "%.1f W", power))
            }
            if let battery = sample.batteryLevel {
                MetricRow(
                    label: "Battery",
                    value: Format.percent(battery) + (sample.batteryCharging ? " charging" : ""))
            }
        }
    }

    private func memorySection(_ sample: Sample) -> some View {
        Section("Memory") {
            PercentBar(
                label: "Use", percent: sample.memoryPercent,
                detail: "\(Format.bytes(unsigned: sample.memoryUsedBytes)) of \(Format.bytes(unsigned: sample.memTotal))")
            MetricRow(
                label: "Swap",
                value: "\(Format.bytes(unsigned: sample.swapUsedBytes)) of \(Format.bytes(unsigned: sample.swapTotal))")
        }
    }

    private func storageSection(_ sample: Sample) -> some View {
        Section("Storage") {
            if sample.disks.isEmpty {
                Text("The agent reports no disk.").foregroundStyle(.secondary)
            }
            ForEach(sample.disks) { disk in
                PercentBar(
                    label: disk.mount, percent: disk.usedPercent,
                    detail: "\(Format.bytes(unsigned: disk.used)) of \(Format.bytes(unsigned: disk.total)) · \(disk.fsType)")
            }
        }
    }

    private func networkSection(_ sample: Sample) -> some View {
        Section("Network") {
            MetricRow(
                label: "Link",
                value: sample.networkInterface.isEmpty ? Format.unknown : sample.networkInterface)
            MetricRow(
                label: "Type", value: sample.networkType.isEmpty ? Format.unknown : sample.networkType)
            MetricRow(
                label: "Speed",
                value: sample.networkLinkMbps > 0 ? "\(sample.networkLinkMbps) Mbps" : Format.unknown)
            MetricRow(label: "Receive", value: Format.rate(bytesPerSecond: sample.netRxRate))
            MetricRow(label: "Send", value: Format.rate(bytesPerSecond: sample.netTxRate))
        }
    }

    private func acceleratorSection(_ sample: Sample) -> some View {
        Section("Accelerators") {
            ForEach(sample.accelerators) { item in
                MetricRow(
                    label: "\(item.kind) \(item.name)",
                    value: Format.optionalPercent(item.utilizationPercent))
            }
        }
    }

    private func processSection(_ sample: Sample) -> some View {
        Section("Top processes") {
            if sample.processes.isEmpty {
                Text("The agent reports no process.").foregroundStyle(.secondary)
            }
            ForEach(sample.topProcesses.prefix(12)) { process in
                VStack(alignment: .leading, spacing: 2) {
                    HStack {
                        Text(process.command).font(.subheadline)
                        Spacer()
                        Text(Format.percent(process.cpu)).monospacedDigit()
                    }
                    Text("pid \(process.pid) · \(process.user) · \(Format.bytes(unsigned: process.rss))")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }
}
