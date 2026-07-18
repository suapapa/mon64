package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/config"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	stats := st.ScrapeStats()
	if stats.NodesConfigured != 1 {
		t.Fatalf("nodes configured = %d", stats.NodesConfigured)
	}
}
