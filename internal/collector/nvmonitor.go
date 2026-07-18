package collector

import (
	"context"
	"io"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

// NvMonitorCollector parses nv-monitor metrics.
// When multiple GPUs are present, GPU utilization is the arithmetic mean
// of nv_gpu_utilization_percent across all gpu labels.
type NvMonitorCollector struct{}

// NewNvMonitorCollector creates an nv-monitor collector.
func NewNvMonitorCollector() *NvMonitorCollector {
	return &NvMonitorCollector{}
}

// Collect implements Collector.
func (c *NvMonitorCollector) Collect(ctx context.Context, cfg config.NodeConfig, body io.Reader, at time.Time) domain.NodeState {
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
		if v, ok := gaugeByName(metrics, "nv_cpu_usage_percent"); ok {
			state.CPU = domain.Ptr(domain.ClampPercent(v))
		} else {
			for key, v := range labeledValues(metrics, "nv_cpu_usage_percent") {
				if cpu, ok := labelValue(key, "cpu"); ok && cpu == "overall" {
					state.CPU = domain.Ptr(domain.ClampPercent(v))
					break
				}
			}
		}
	}
	if cfg.Wants(config.CollectGPU) {
		gpus := labeledValues(metrics, "nv_gpu_utilization_percent")
		if len(gpus) > 0 {
			var sum float64
			for _, v := range gpus {
				sum += v
			}
			avg := domain.ClampPercent(sum / float64(len(gpus)))
			state.GPU = domain.Ptr(avg)
		}
	}
	if cfg.Wants(config.CollectMem) {
		total, okT := gaugeByName(metrics, "nv_memory_total_bytes")
		if okT && total > 0 {
			if used, ok := gaugeByName(metrics, "nv_memory_used_bytes"); ok {
				state.MemUsed = domain.Ptr(domain.ClampPercent(used / total * 100))
			}
			if buf, ok := gaugeByName(metrics, "nv_memory_bufcache_bytes"); ok {
				state.MemCached = domain.Ptr(domain.ClampPercent(buf / total * 100))
			}
		}
	}
	if cfg.Wants(config.CollectSwap) {
		total, okT := gaugeByName(metrics, "nv_swap_total_bytes")
		used, okU := gaugeByName(metrics, "nv_swap_used_bytes")
		if okT && okU && total > 0 {
			state.SwapUsed = domain.Ptr(domain.ClampPercent(used / total * 100))
		}
	}
	return state
}
