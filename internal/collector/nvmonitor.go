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

// newNvMonitorCollector creates an nv-monitor collector.
func newNvMonitorCollector() *NvMonitorCollector {
	return &NvMonitorCollector{}
}

// Collect implements Collector.
func (c *NvMonitorCollector) Collect(
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
		if v, ok := gaugeByName(metrics, "nv_cpu_usage_percent"); ok {
			state.CPU = new(domain.ClampPercent(v))
		} else {
			for key, v := range labeledValues(metrics, "nv_cpu_usage_percent") {
				if cpu, ok := labelValue(key, "cpu"); ok && cpu == "overall" {
					state.CPU = new(domain.ClampPercent(v))
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
			state.GPU = new(avg)
		}
	}
	if cfg.Wants(config.CollectMem) {
		total, okT := gaugeByName(metrics, "nv_memory_total_bytes")
		haveMemTotal := okT && total > 0
		if haveMemTotal {
			if used, ok := gaugeByName(metrics, "nv_memory_used_bytes"); ok {
				state.MemUsed = new(domain.ClampPercent(used / total * 100))
			}
			if buf, ok := gaugeByName(metrics, "nv_memory_bufcache_bytes"); ok {
				state.MemCached = new(domain.ClampPercent(buf / total * 100))
			}
		}
	}
	if cfg.Wants(config.CollectSwap) {
		total, okT := gaugeByName(metrics, "nv_swap_total_bytes")
		used, okU := gaugeByName(metrics, "nv_swap_used_bytes")
		haveSwap := okT && okU && total > 0
		if haveSwap {
			state.SwapUsed = new(domain.ClampPercent(used / total * 100))
		}
	}
	return state
}
