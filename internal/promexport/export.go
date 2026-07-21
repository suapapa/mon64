// Package promexport serves normalized node metrics as Prometheus text on
// dedicated listen ports configured under exports.prometheuses.
package promexport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

type snapshotSource interface {
	Snapshot() domain.Snapshot
}

// Exporter runs one HTTP listener per PrometheusExport entry.
type Exporter struct {
	source snapshotSource
	log    *slog.Logger
	cfgs   []config.PrometheusExport
}

// New builds an exporter for the given Prometheus export configs.
func New(source snapshotSource, cfgs []config.PrometheusExport, log *slog.Logger) (*Exporter, error) {
	if source == nil {
		return nil, fmt.Errorf("promexport: snapshot source is required")
	}
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("promexport: at least one export is required")
	}
	if log == nil {
		log = slog.Default()
	}
	copied := make([]config.PrometheusExport, len(cfgs))
	for i, c := range cfgs {
		addr, err := config.NormalizeListenPort(c.Port)
		if err != nil {
			return nil, fmt.Errorf("promexport: exports[%d].port: %w", i, err)
		}
		nodes := append([]string(nil), c.Nodes...)
		copied[i] = config.PrometheusExport{Port: addr, Nodes: nodes}
	}
	return &Exporter{source: source, log: log, cfgs: copied}, nil
}

// Run listens until ctx is cancelled, then shuts down all servers.
func (e *Exporter) Run(ctx context.Context) {
	var wg sync.WaitGroup
	servers := make([]*http.Server, 0, len(e.cfgs))

	for _, cfg := range e.cfgs {
		cfg := cfg
		mux := http.NewServeMux()
		mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			WritePrometheus(w, e.source.Snapshot(), cfg.Nodes)
		})
		srv := &http.Server{
			Addr:              cfg.Port,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		servers = append(servers, srv)

		wg.Add(1)
		go func(addr string, nodeNames []string) {
			defer wg.Done()
			e.log.Info("prometheus export listening", "addr", addr, "nodes", nodeNames)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				e.log.Error("prometheus export listen failed", "addr", addr, "err", err)
			}
		}(cfg.Port, cfg.Nodes)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			e.log.Error("prometheus export shutdown failed", "addr", srv.Addr, "err", err)
		}
	}
	wg.Wait()
}

// WritePrometheus renders normalized metrics for the listed node names only.
// Nodes absent from the snapshot are omitted. Nil percent fields are omitted.
func WritePrometheus(w io.Writer, snap domain.Snapshot, nodeNames []string) {
	byName := make(map[string]domain.NodeState, len(snap.Nodes))
	for _, n := range snap.Nodes {
		byName[n.Name] = n
	}

	nodes := make([]domain.NodeState, 0, len(nodeNames))
	for _, name := range nodeNames {
		n, ok := byName[name]
		if !ok {
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return
	}

	// Stable series order: config node order, then metric families below.
	writeGaugeFamily(w, "mon64_node_reachable",
		"Whether the node was reachable on the last scrape (1=yes, 0=no).",
		nodes, func(n domain.NodeState) (float64, bool) {
			if n.Reachable {
				return 1, true
			}
			return 0, true
		})
	writeGaugeFamily(w, "mon64_node_collected_timestamp_seconds",
		"Unix time of the node's last collection.",
		nodes, func(n domain.NodeState) (float64, bool) {
			if n.CollectedAt.IsZero() {
				return 0, false
			}
			return float64(n.CollectedAt.UnixNano()) / 1e9, true
		})
	writeGaugeFamily(w, "mon64_node_cpu_percent",
		"CPU usage percent.",
		nodes, ptrMetric(func(n domain.NodeState) *float64 { return n.CPU }))
	writeGaugeFamily(w, "mon64_node_gpu_percent",
		"GPU utilization percent (average across GPUs when multiple).",
		nodes, ptrMetric(func(n domain.NodeState) *float64 { return n.GPU }))
	writeGaugeFamily(w, "mon64_node_mem_used_percent",
		"Memory used percent.",
		nodes, ptrMetric(func(n domain.NodeState) *float64 { return n.MemUsed }))
	writeGaugeFamily(w, "mon64_node_mem_cached_percent",
		"Memory cached percent.",
		nodes, ptrMetric(func(n domain.NodeState) *float64 { return n.MemCached }))
	writeGaugeFamily(w, "mon64_node_swap_used_percent",
		"Swap used percent.",
		nodes, ptrMetric(func(n domain.NodeState) *float64 { return n.SwapUsed }))
}

func ptrMetric(get func(domain.NodeState) *float64) func(domain.NodeState) (float64, bool) {
	return func(n domain.NodeState) (float64, bool) {
		p := get(n)
		if p == nil {
			return 0, false
		}
		return *p, true
	}
}

func writeGaugeFamily(
	w io.Writer,
	name, help string,
	nodes []domain.NodeState,
	value func(domain.NodeState) (float64, bool),
) {
	type sample struct {
		node  string
		value float64
	}
	samples := make([]sample, 0, len(nodes))
	for _, n := range nodes {
		v, ok := value(n)
		if !ok {
			continue
		}
		samples = append(samples, sample{node: n.Name, value: v})
	}
	if len(samples) == 0 {
		return
	}
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", name)
	for _, s := range samples {
		fmt.Fprintf(w, "%s{node=%q} %s\n",
			name, s.node, strconv.FormatFloat(s.value, 'f', -1, 64))
	}
}
