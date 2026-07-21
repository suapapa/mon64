package store_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/store"
)

func TestReload(t *testing.T) {
	cfg := &config.Config{
		Listen:         ":8080",
		ScrapeInterval: time.Minute,
		ScrapeTimeout:  time.Second,
		Nodes: []config.NodeConfig{{
			Name:         "a",
			PromFmt:      config.PromFmtNodeExporter,
			PromEndpoint: "http://127.0.0.1:1",
			Collects:     []config.CollectKind{config.CollectCPU},
		}},
	}
	st := store.New(cfg)

	newCfg := &config.Config{
		Listen:         ":8080",
		ScrapeInterval: 30 * time.Second,
		ScrapeTimeout:  2 * time.Second,
		Nodes: []config.NodeConfig{
			{
				Name:         "b",
				PromFmt:      config.PromFmtNodeExporter,
				PromEndpoint: "http://127.0.0.1:2",
				Collects:     []config.CollectKind{config.CollectMem},
			},
		},
	}
	if err := st.Reload(newCfg); err != nil {
		t.Fatal(err)
	}
	if err := st.Reload(&config.Config{}); err == nil {
		t.Fatal("expected validation error")
	}

	ctx := t.Context()
	go st.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	stats := st.ScrapeStats()
	if stats.NodesConfigured != 1 {
		t.Fatalf("nodes configured = %d", stats.NodesConfigured)
	}
}

func TestSnapshotPreservesConfigOrder(t *testing.T) {
	cfg := &config.Config{
		Listen:         ":8080",
		ScrapeInterval: time.Hour,
		ScrapeTimeout:  time.Second,
		Nodes: []config.NodeConfig{
			{
				Name:     "alpha",
				PromFmt:  config.PromFmtNodeExporter,
				Collects: []config.CollectKind{config.CollectCPU},
			},
			{
				Name:     "bravo",
				PromFmt:  config.PromFmtNvMonitor,
				Collects: []config.CollectKind{config.CollectGPU},
			},
			{
				Name:     "charlie",
				PromFmt:  config.PromFmtNodeExporter,
				Collects: []config.CollectKind{config.CollectMem},
			},
		},
	}
	st := store.NewDummy(cfg)
	ctx := t.Context()
	go st.Start(ctx)

	deadline := time.Now().Add(time.Second)
	var snap domain.Snapshot
	for time.Now().Before(deadline) {
		snap = st.Snapshot()
		if len(snap.Nodes) == 3 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(snap.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(snap.Nodes))
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i, name := range want {
		if snap.Nodes[i].Name != name {
			t.Fatalf("nodes[%d] = %q, want %q (full=%v)",
				i, snap.Nodes[i].Name, name, names(snap.Nodes))
		}
	}
}

func names(nodes []domain.NodeState) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

func TestDummyStoreUsesRequestedMetricsWithoutScraping(t *testing.T) {
	var requests atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer endpoint.Close()

	cfg := &config.Config{
		Listen:         ":8080",
		ScrapeInterval: time.Hour,
		ScrapeTimeout:  time.Second,
		Nodes: []config.NodeConfig{{
			Name:         "demo",
			PromEndpoint: endpoint.URL,
			Collects: []config.CollectKind{
				config.CollectCPU,
				config.CollectMem,
			},
		}},
	}
	st := store.NewDummy(cfg)
	ctx := t.Context()
	go st.Start(ctx)

	deadline := time.Now().Add(time.Second)
	var nodeFound bool
	for time.Now().Before(deadline) {
		node, ok := st.NodeByName("demo")
		if ok {
			nodeFound = true
			if !node.Reachable {
				t.Fatal("dummy node is unreachable")
			}
			if node.CPU == nil || node.MemUsed == nil || node.MemCached == nil {
				t.Fatalf("requested metrics missing: %#v", node)
			}
			if node.GPU != nil || node.SwapUsed != nil {
				t.Fatalf("unrequested metrics populated: %#v", node)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !nodeFound {
		t.Fatal("dummy node was not collected")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("Prometheus endpoint requests = %d, want 0", got)
	}
}
