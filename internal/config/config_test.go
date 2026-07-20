package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/config"
)

func TestEffectiveScrapeInterval(t *testing.T) {
	defaultInterval := 15 * time.Second
	node := config.NodeConfig{Name: "a", ScrapeInterval: 30 * time.Second}
	if got := node.EffectiveScrapeInterval(defaultInterval); got != 30*time.Second {
		t.Fatalf("custom interval = %v", got)
	}
	defaultNode := config.NodeConfig{Name: "b"}
	if got := defaultNode.EffectiveScrapeInterval(defaultInterval); got != defaultInterval {
		t.Fatalf("default interval = %v", got)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "example.yaml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
	if cfg.ScrapeInterval != 15*time.Second {
		t.Fatalf("interval = %v", cfg.ScrapeInterval)
	}
	if len(cfg.Nodes) != 3 {
		t.Fatalf("nodes = %d", len(cfg.Nodes))
	}
	if len(cfg.Badges) != 1 || cfg.Badges[0].Name != "homelab" {
		t.Fatalf("badges = %#v", cfg.Badges)
	}
	if cfg.Badges[0].Type != config.BadgeTypeRect64 {
		t.Fatalf("badge type = %q", cfg.Badges[0].Type)
	}
	if len(cfg.Exports.Pixoo64) != 1 || cfg.Exports.Pixoo64[0].Badge != "homelab" {
		t.Fatalf("exports.pixoo64 = %#v", cfg.Exports.Pixoo64)
	}
}

func TestLoadDummyOnlyRequiresNodeNameAndCollects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dummy.yaml")
	data := []byte(`
listen: ":8080"
scrape_interval: 15s
scrape_timeout: 5s
nodes:
  - name: demo
    collects: [cpu, gpu, mem, swap]
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadDummy(path)
	if err != nil {
		t.Fatalf("LoadDummy: %v", err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "demo" {
		t.Fatalf("nodes = %#v", cfg.Nodes)
	}
	if _, err := config.Load(path); err == nil {
		t.Fatal("regular Load accepted missing Prometheus settings")
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"empty nodes", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes: []\n"},
		{"bad prom_fmt", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: bad\n    prom_endpoint: http://x\n    collects: [cpu]\n"},
		{"gpu on node-exporter", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [gpu]\n"},
		{"timeout >= interval", "listen: \":8080\"\nscrape_interval: 5s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [cpu]\n"},
		{"node timeout >= node interval", "listen: \":8080\"\nscrape_interval: 60s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    scrape_interval: 3s\n    collects: [cpu]\n"},
		{"unknown badge type", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [cpu]\nbadges:\n  - name: b\n    type: nope\n    nodes: [x]\n"},
		{"circle128 not implemented", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [cpu]\nbadges:\n  - name: b\n    type: circle128\n    nodes: [x]\n"},
		{"badge unknown node", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [cpu]\nbadges:\n  - name: b\n    type: rect64\n    nodes: [missing]\n"},
		{"export badge mismatch", "listen: \":8080\"\nscrape_interval: 15s\nscrape_timeout: 5s\nnodes:\n  - name: x\n    prom_fmt: node-exporter\n    prom_endpoint: http://x\n    collects: [cpu]\nbadges:\n  - name: b\n    type: rect64\n    nodes: [x]\nexports:\n  pixoo64:\n    - badge: b\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString(tc.yaml); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if _, err := config.Load(f.Name()); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
