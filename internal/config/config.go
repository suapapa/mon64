package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	PromFmtNodeExporter = "node-exporter"
	PromFmtNvMonitor    = "nv-monitor"
)

// CollectKind identifies which metrics to derive for a node.
type CollectKind string

const (
	CollectCPU  CollectKind = "cpu"
	CollectGPU  CollectKind = "gpu"
	CollectMem  CollectKind = "mem"
	CollectSwap CollectKind = "swap"
)

// Config holds the full application configuration.
type Config struct {
	Listen         string        `yaml:"listen"`
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
	ScrapeTimeout  time.Duration `yaml:"scrape_timeout"`
	Nodes          []NodeConfig  `yaml:"nodes"`
}

// NodeConfig describes one monitored endpoint.
type NodeConfig struct {
	Name           string        `yaml:"name"`
	PromFmt        string        `yaml:"prom_fmt"`
	PromEndpoint   string        `yaml:"prom_endpoint"`
	PromAPIKey     string        `yaml:"prom_api_key"`
	ScrapeInterval time.Duration `yaml:"scrape_interval,omitempty"`
	Collects       []CollectKind `yaml:"collects"`
}

// Load reads and validates configuration from path.
func Load(path string) (*Config, error) {
	return load(path, false)
}

// LoadDummy reads configuration for dummy mode. Node Prometheus settings are
// ignored, so only node names and requested collections are validated.
func LoadDummy(path string) (*Config, error) {
	return load(path, true)
}

func load(path string, dummy bool) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(dummy); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks required fields and allowed values.
func (c *Config) Validate() error {
	return c.validate(false)
}

// ValidateDummy checks configuration used in dummy mode.
func (c *Config) ValidateDummy() error {
	return c.validate(true)
}

func (c *Config) validate(dummy bool) error {
	if c.Listen == "" {
		return fmt.Errorf("listen is required")
	}
	if c.ScrapeInterval <= 0 {
		return fmt.Errorf("scrape_interval must be positive")
	}
	if c.ScrapeTimeout <= 0 {
		return fmt.Errorf("scrape_timeout must be positive")
	}
	if len(c.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}
	names := make(map[string]struct{}, len(c.Nodes))
	for i, n := range c.Nodes {
		if n.Name == "" {
			return fmt.Errorf("nodes[%d]: name is required", i)
		}
		if _, dup := names[n.Name]; dup {
			return fmt.Errorf("nodes[%d]: duplicate name %q", i, n.Name)
		}
		names[n.Name] = struct{}{}
		if len(n.Collects) == 0 {
			return fmt.Errorf("nodes[%d]: collects must not be empty", i)
		}
		for j, k := range n.Collects {
			switch k {
			case CollectCPU, CollectGPU, CollectMem, CollectSwap:
			default:
				return fmt.Errorf("nodes[%d].collects[%d]: unknown kind %q", i, j, k)
			}
		}
		if dummy {
			continue
		}
		if n.PromFmt != PromFmtNodeExporter && n.PromFmt != PromFmtNvMonitor {
			return fmt.Errorf("nodes[%d]: prom_fmt must be %q or %q", i, PromFmtNodeExporter, PromFmtNvMonitor)
		}
		if n.PromEndpoint == "" {
			return fmt.Errorf("nodes[%d]: prom_endpoint is required", i)
		}
		if n.PromFmt == PromFmtNodeExporter {
			for _, k := range n.Collects {
				if k == CollectGPU {
					return fmt.Errorf("nodes[%d]: node-exporter does not support gpu collection", i)
				}
			}
		}
		interval := n.EffectiveScrapeInterval(c.ScrapeInterval)
		if c.ScrapeTimeout >= interval {
			return fmt.Errorf("nodes[%d]: scrape_timeout must be less than scrape_interval (%v)", i, interval)
		}
	}
	return nil
}

// EffectiveScrapeInterval returns the node interval or the global default.
func (n NodeConfig) EffectiveScrapeInterval(defaultInterval time.Duration) time.Duration {
	if n.ScrapeInterval > 0 {
		return n.ScrapeInterval
	}
	return defaultInterval
}

// Wants reports whether the node config requests kind.
func (n NodeConfig) Wants(kind CollectKind) bool {
	for _, k := range n.Collects {
		if k == kind {
			return true
		}
	}
	return false
}

// CollectStrings returns collects as plain strings for NodeState.
func (n NodeConfig) CollectStrings() []string {
	out := make([]string, len(n.Collects))
	for i, k := range n.Collects {
		out[i] = string(k)
	}
	return out
}
