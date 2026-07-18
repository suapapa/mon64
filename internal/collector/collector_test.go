package collector_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/collector"
	"github.com/suapapa/mon64/internal/config"
)

func fixture(name string) string {
	path := filepath.Join("..", "..", "ref", name)
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestNodeExporterFixture(t *testing.T) {
	engine := collector.NewEngine(time.Second)
	node := config.NodeConfig{
		Name:    "omv",
		PromFmt: config.PromFmtNodeExporter,
		Collects: []config.CollectKind{
			config.CollectCPU, config.CollectMem, config.CollectSwap,
		},
	}
	at := time.Now().UTC()

	// First scrape: CPU unavailable (no prior sample).
	s1 := engine.CollectFromReader(node, strings.NewReader(fixture("omv_node-exporter_metrics.metrics")), at)
	if s1.CPU != nil {
		t.Fatalf("first CPU should be nil, got %v", *s1.CPU)
	}
	if s1.MemUsed == nil || s1.MemCached == nil {
		t.Fatal("expected memory values")
	}
	if s1.SwapUsed == nil {
		t.Fatal("expected swap value")
	}

	// Second scrape with bumped counters: CPU available.
	bumped := strings.Replace(fixture("omv_node-exporter_metrics.metrics"), `mode="idle"} 78995.73`, `mode="idle"} 79095.73`, 1)
	s2 := engine.CollectFromReader(node, strings.NewReader(bumped), at.Add(time.Second))
	if s2.CPU == nil {
		t.Fatal("expected CPU on second scrape")
	}
	if *s2.CPU < 0 || *s2.CPU > 100 {
		t.Fatalf("CPU out of range: %v", *s2.CPU)
	}
}

func TestNodeExporterMemorySwapEdgeCases(t *testing.T) {
	c := collector.NewNodeExporterCollector()
	node := config.NodeConfig{
		Name:     "test",
		PromFmt:  config.PromFmtNodeExporter,
		Collects: []config.CollectKind{config.CollectMem, config.CollectSwap},
	}
	body := strings.NewReader(`
node_memory_MemTotal_bytes 100
node_memory_MemAvailable_bytes 40
node_memory_Cached_bytes 10
node_memory_SwapTotal_bytes 0
node_memory_SwapFree_bytes 0
`)
	state := c.Collect(t.Context(), node, body, time.Now())
	if state.MemUsed == nil || *state.MemUsed != 60 {
		t.Fatalf("mem used = %v", state.MemUsed)
	}
	if state.MemCached == nil || *state.MemCached != 10 {
		t.Fatalf("mem cached = %v", state.MemCached)
	}
	if state.SwapUsed != nil {
		t.Fatalf("swap should be nil when total is 0, got %v", *state.SwapUsed)
	}
}

func TestNvMonitorFixture(t *testing.T) {
	engine := collector.NewEngine(time.Second)
	node := config.NodeConfig{
		Name:    "spark",
		PromFmt: config.PromFmtNvMonitor,
		Collects: []config.CollectKind{
			config.CollectCPU, config.CollectGPU, config.CollectMem, config.CollectSwap,
		},
	}
	state := engine.CollectFromReader(node, strings.NewReader(fixture("spark_nv-monitor.metrics")), time.Now())
	if !state.Reachable {
		t.Fatalf("unreachable: %s", state.LastError)
	}
	if state.CPU == nil {
		t.Fatal("expected CPU")
	}
	if *state.CPU != 0.2 {
		t.Fatalf("cpu = %v", *state.CPU)
	}
	if state.GPU == nil || *state.GPU != 0 {
		t.Fatalf("gpu = %v", state.GPU)
	}
	if state.MemUsed == nil || state.MemCached == nil || state.SwapUsed == nil {
		t.Fatal("expected mem/swap values")
	}
}

func TestVraptorFixture(t *testing.T) {
	engine := collector.NewEngine(time.Second)
	node := config.NodeConfig{
		Name:     "vraptor",
		PromFmt:  config.PromFmtNodeExporter,
		Collects: []config.CollectKind{config.CollectCPU, config.CollectMem},
	}
	state := engine.CollectFromReader(node, strings.NewReader(fixture("vraptor_node-exporter_metrics.metrics")), time.Now())
	if !state.Reachable {
		t.Fatalf("unreachable: %s", state.LastError)
	}
	if state.MemUsed == nil {
		t.Fatal("expected mem used")
	}
	if state.GPU != nil {
		t.Fatal("gpu must be nil when not collected")
	}
	if !state.Wants("cpu") || state.Wants("gpu") {
		t.Fatalf("collects = %v", state.Collects)
	}
}

func TestMalformedMetrics(t *testing.T) {
	engine := collector.NewEngine(time.Second)
	node := config.NodeConfig{
		Name:     "bad",
		PromFmt:  config.PromFmtNodeExporter,
		Collects: []config.CollectKind{config.CollectCPU},
	}
	state := engine.CollectFromReader(node, strings.NewReader("not prometheus {"), time.Now())
	if state.Reachable {
		t.Fatal("expected unreachable for malformed metrics")
	}
}

func TestUnreachableEndpoint(t *testing.T) {
	engine := collector.NewEngine(100 * time.Millisecond)
	node := config.NodeConfig{
		Name:         "down",
		PromFmt:      config.PromFmtNodeExporter,
		PromEndpoint: "http://127.0.0.1:1",
		Collects:     []config.CollectKind{config.CollectCPU},
	}
	states := engine.CollectAll(t.Context(), []config.NodeConfig{node})
	if len(states) != 1 {
		t.Fatalf("states = %d", len(states))
	}
	if states[0].Reachable {
		t.Fatal("expected unreachable")
	}
	if states[0].LastError == "" {
		t.Fatal("expected error message")
	}
}
