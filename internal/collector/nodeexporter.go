package collector

import (
	"context"
	"io"
	"maps"
	"sync"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

// Collector derives normalized node state from Prometheus text.
type Collector interface {
	Collect(
		ctx context.Context,
		cfg config.NodeConfig,
		body io.Reader,
		at time.Time,
	) domain.NodeState
}

// NodeExporterCollector parses node_exporter metrics.
// CPU usage requires a prior sample; first scrape leaves CPU unavailable.
type NodeExporterCollector struct {
	mu   sync.Mutex
	prev map[string]cpuSnapshot
}

type cpuSnapshot struct {
	at    time.Time
	byKey map[string]float64
}

// NewNodeExporterCollector creates a collector with per-node CPU history.
func NewNodeExporterCollector() *NodeExporterCollector {
	return &NodeExporterCollector{prev: make(map[string]cpuSnapshot)}
}

// Collect implements Collector.
func (c *NodeExporterCollector) Collect(
	ctx context.Context,
	cfg config.NodeConfig,
	body io.Reader,
	at time.Time,
) domain.NodeState {
	state := domain.NodeState{
		Name:        cfg.Name,
		CollectedAt: at,
		Reachable:   true,
		Collects:    cfg.CollectStrings(),
	}
	metrics, err := parseMetrics(body)
	if err != nil {
		state.Reachable = false
		state.LastError = err.Error()
		return state
	}
	if cfg.Wants(config.CollectCPU) {
		if cpu := c.cpuUsage(cfg.Name, metrics); cpu != nil {
			state.CPU = cpu
		}
	}
	if cfg.Wants(config.CollectMem) {
		if used, cached := memPercents(metrics); used != nil || cached != nil {
			state.MemUsed = used
			state.MemCached = cached
		}
	}
	if cfg.Wants(config.CollectSwap) {
		state.SwapUsed = swapUsed(metrics)
	}
	return state
}

func (c *NodeExporterCollector) cpuUsage(node string, metrics map[string]float64) *float64 {
	cpuMetrics := labeledValues(metrics, "node_cpu_seconds_total")
	if len(cpuMetrics) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	prev, hasPrev := c.prev[node]
	c.prev[node] = cpuSnapshot{at: time.Now(), byKey: cloneMap(cpuMetrics)}
	if !hasPrev {
		return nil
	}
	var idleDelta, totalDelta float64
	for key, cur := range cpuMetrics {
		pv, ok := prev.byKey[key]
		if !ok {
			continue
		}
		d := cur - pv
		if d < 0 {
			continue
		}
		totalDelta += d
		if mode, ok := labelValue(key, "mode"); ok && mode == "idle" {
			idleDelta += d
		}
	}
	if totalDelta <= 0 {
		return nil
	}
	usage := domain.ClampPercent(100 * (1 - idleDelta/totalDelta))
	return new(usage)
}

func memPercents(metrics map[string]float64) (*float64, *float64) {
	total, okT := gaugeByName(metrics, "node_memory_MemTotal_bytes")
	if !okT || total <= 0 {
		return nil, nil
	}

	var usedBytes float64
	haveUsed := false
	if avail, ok := gaugeByName(metrics, "node_memory_MemAvailable_bytes"); ok {
		usedBytes = total - avail
		haveUsed = true
	}

	var cachedBytes float64
	haveCached := false
	if c, ok := gaugeByName(metrics, "node_memory_Cached_bytes"); ok {
		cachedBytes = c
		haveCached = true
	}

	// ZFS ARC is reclaimable cache but is not reflected in MemAvailable or
	// Cached; treat it as cache so mem_used shows application pressure only.
	if arc, ok := gaugeByName(metrics, "node_zfs_arc_size"); ok && arc > 0 {
		if haveUsed {
			usedBytes -= arc
			if usedBytes < 0 {
				usedBytes = 0
			}
		}
		cachedBytes += arc
		haveCached = true
	}

	var used, cached *float64
	if haveUsed {
		used = new(domain.ClampPercent(usedBytes / total * 100))
	}
	if haveCached {
		cached = new(domain.ClampPercent(cachedBytes / total * 100))
	}
	return used, cached
}

func swapUsed(metrics map[string]float64) *float64 {
	total, okT := gaugeByName(metrics, "node_memory_SwapTotal_bytes")
	free, okF := gaugeByName(metrics, "node_memory_SwapFree_bytes")
	haveSwap := okT && okF && total > 0
	if !haveSwap {
		return nil
	}
	u := domain.ClampPercent((total - free) / total * 100)
	return new(u)
}

func cloneMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	maps.Copy(out, in)
	return out
}
