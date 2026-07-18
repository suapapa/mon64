package collector

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

// Collector derives normalized node state from Prometheus text.
type Collector interface {
	Collect(ctx context.Context, cfg config.NodeConfig, body io.Reader, at time.Time) domain.NodeState
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
func (c *NodeExporterCollector) Collect(ctx context.Context, cfg config.NodeConfig, body io.Reader, at time.Time) domain.NodeState {
	state := domain.NodeState{
		Name:        cfg.Name,
		CollectedAt: at,
		Reachable:   true,
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
	return domain.Ptr(usage)
}

func memPercents(metrics map[string]float64) (*float64, *float64) {
	total, okT := gaugeByName(metrics, "node_memory_MemTotal_bytes")
	if !okT || total <= 0 {
		return nil, nil
	}
	var used, cached *float64
	if avail, ok := gaugeByName(metrics, "node_memory_MemAvailable_bytes"); ok {
		u := domain.ClampPercent((total - avail) / total * 100)
		used = domain.Ptr(u)
	}
	if c, ok := gaugeByName(metrics, "node_memory_Cached_bytes"); ok {
		cached = domain.Ptr(domain.ClampPercent(c / total * 100))
	}
	return used, cached
}

func swapUsed(metrics map[string]float64) *float64 {
	total, okT := gaugeByName(metrics, "node_memory_SwapTotal_bytes")
	free, okF := gaugeByName(metrics, "node_memory_SwapFree_bytes")
	if !okT || !okF || total <= 0 {
		return nil
	}
	u := domain.ClampPercent((total - free) / total * 100)
	return domain.Ptr(u)
}

func cloneMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
