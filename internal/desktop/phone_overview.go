package desktop

import (
	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
	"github.com/dacsang97/nakube/internal/views/overview"
)

func phoneOverview(name string, shown overview.Snapshot, usage cluster.Usage) remote.Overview {
	charts := &remote.OverviewCharts{
		NodesReady: shown.NodesReady, NodesTotal: shown.NodesTotal,
		PodsRunning: shown.PodsRunning, PodsTotal: shown.PodsTotal,
	}
	for _, node := range shown.Nodes {
		charts.CPU.Allocatable += node.CPUAlloc
		charts.Memory.Allocatable += node.MemAlloc
	}
	// A partial metrics response cannot represent cluster-wide utilization.
	if shown.MetricsOK && len(usage.Nodes) == shown.NodesTotal && shown.NodesTotal > 0 {
		complete := true
		// Both snapshot facts and metrics samples are sorted by node name.
		for i, node := range shown.Nodes {
			complete = complete && usage.Nodes[i].Name == node.Name
		}
		if complete {
			cpu, memory := usage.CPUMilli, usage.MemoryBytes
			charts.CPU.Used, charts.Memory.Used = &cpu, &memory
		}
	}
	return remote.Overview{
		Context: name, Nodes: shown.NodesText(), Pods: shown.PodsText(), Warnings: shown.WarningsText(),
		CPU: shown.CPU, Memory: shown.Memory, Charts: charts,
	}
}
