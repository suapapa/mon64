package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	PromFmtNodeExporter = "node-exporter"
	PromFmtNvMonitor    = "nv-monitor"

	BadgeTypeRect64 = "rect64"
	// BadgeTypeCircle128 is reserved; not implemented yet.
	BadgeTypeCircle128 = "circle128"
	BadgeTypeCircle240 = "circle240"

	ExportPixoo64 = "pixoo64"
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
	Exports        ExportsConfig `yaml:"exports"`
	Badges         []BadgeConfig `yaml:"badges"`
	Nodes          []NodeConfig  `yaml:"nodes"`
}

// ExportsConfig binds named badges and Prometheus listeners to targets.
type ExportsConfig struct {
	Pixoo64      []Pixoo64Export    `yaml:"pixoo64"`
	Prometheuses []PrometheusExport `yaml:"prometheuses"`
}

// Pixoo64Export pushes one named badge to a discovered Pixoo64.
type Pixoo64Export struct {
	Badge string `yaml:"badge"`
}

// PrometheusExport serves normalized node metrics on a dedicated port.
type PrometheusExport struct {
	Port  string   `yaml:"port"`
	Nodes []string `yaml:"nodes"`
}

// BadgeConfig describes a named composite badge.
type BadgeConfig struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Nodes   []string `yaml:"nodes"`
	Exports []string `yaml:"exports"`
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
			if slices.Contains(n.Collects, CollectGPU) {
				return fmt.Errorf("nodes[%d]: node-exporter does not support gpu collection", i)
			}
		}
		interval := n.EffectiveScrapeInterval(c.ScrapeInterval)
		if c.ScrapeTimeout >= interval {
			return fmt.Errorf("nodes[%d]: scrape_timeout must be less than scrape_interval (%v)", i, interval)
		}
	}
	return c.validateBadgesAndExports(names)
}

func (c *Config) validateBadgesAndExports(nodeNames map[string]struct{}) error {
	badgeNames := make(map[string]struct{}, len(c.Badges))
	badgeExports := make(map[string]map[string]struct{}, len(c.Badges))

	for i, b := range c.Badges {
		if b.Name == "" {
			return fmt.Errorf("badges[%d]: name is required", i)
		}
		if _, dup := badgeNames[b.Name]; dup {
			return fmt.Errorf("badges[%d]: duplicate name %q", i, b.Name)
		}
		badgeNames[b.Name] = struct{}{}

		switch b.Type {
		case BadgeTypeRect64, BadgeTypeCircle240:
		case BadgeTypeCircle128:
			return fmt.Errorf("badges[%d]: type %q is not implemented yet", i, b.Type)
		case "":
			return fmt.Errorf("badges[%d]: type is required", i)
		default:
			return fmt.Errorf("badges[%d]: unknown type %q", i, b.Type)
		}

		if len(b.Nodes) == 0 {
			return fmt.Errorf("badges[%d]: nodes must not be empty", i)
		}
		if b.Type == BadgeTypeCircle240 && len(b.Nodes) != 1 {
			return fmt.Errorf("badges[%d]: type %q requires exactly one node", i, b.Type)
		}
		seenNodes := make(map[string]struct{}, len(b.Nodes))
		for j, nodeName := range b.Nodes {
			if nodeName == "" {
				return fmt.Errorf("badges[%d].nodes[%d]: name is required", i, j)
			}
			if _, ok := nodeNames[nodeName]; !ok {
				return fmt.Errorf("badges[%d].nodes[%d]: unknown node %q", i, j, nodeName)
			}
			if _, dup := seenNodes[nodeName]; dup {
				return fmt.Errorf("badges[%d].nodes[%d]: duplicate node %q", i, j, nodeName)
			}
			seenNodes[nodeName] = struct{}{}
		}

		exps := make(map[string]struct{}, len(b.Exports))
		for j, exp := range b.Exports {
			switch exp {
			case ExportPixoo64:
			case "":
				return fmt.Errorf("badges[%d].exports[%d]: name is required", i, j)
			default:
				return fmt.Errorf("badges[%d].exports[%d]: unknown export %q", i, j, exp)
			}
			if _, dup := exps[exp]; dup {
				return fmt.Errorf("badges[%d].exports[%d]: duplicate export %q", i, j, exp)
			}
			exps[exp] = struct{}{}
		}
		badgeExports[b.Name] = exps
	}

	seenPixooBadges := make(map[string]struct{}, len(c.Exports.Pixoo64))
	for i, exp := range c.Exports.Pixoo64 {
		if exp.Badge == "" {
			return fmt.Errorf("exports.pixoo64[%d]: badge is required", i)
		}
		if _, ok := badgeNames[exp.Badge]; !ok {
			return fmt.Errorf("exports.pixoo64[%d]: unknown badge %q", i, exp.Badge)
		}
		if _, dup := seenPixooBadges[exp.Badge]; dup {
			return fmt.Errorf("exports.pixoo64[%d]: duplicate badge %q", i, exp.Badge)
		}
		seenPixooBadges[exp.Badge] = struct{}{}

		if _, ok := badgeExports[exp.Badge][ExportPixoo64]; !ok {
			return fmt.Errorf(
				"exports.pixoo64[%d]: badge %q must list %q under badges[].exports",
				i, exp.Badge, ExportPixoo64,
			)
		}
	}

	for name, exps := range badgeExports {
		if _, wants := exps[ExportPixoo64]; !wants {
			continue
		}
		if _, ok := seenPixooBadges[name]; !ok {
			return fmt.Errorf(
				"badge %q lists export %q but is missing from exports.pixoo64",
				name, ExportPixoo64,
			)
		}
	}

	return c.validatePrometheusExports(nodeNames)
}

func (c *Config) validatePrometheusExports(nodeNames map[string]struct{}) error {
	seenPorts := make(map[string]struct{}, len(c.Exports.Prometheuses))
	for i, exp := range c.Exports.Prometheuses {
		addr, err := NormalizeListenPort(exp.Port)
		if err != nil {
			return fmt.Errorf("exports.prometheuses[%d].port: %w", i, err)
		}
		if _, dup := seenPorts[addr]; dup {
			return fmt.Errorf("exports.prometheuses[%d]: duplicate port %q", i, addr)
		}
		seenPorts[addr] = struct{}{}
		c.Exports.Prometheuses[i].Port = addr

		if len(exp.Nodes) == 0 {
			return fmt.Errorf("exports.prometheuses[%d]: nodes must not be empty", i)
		}
		seenNodes := make(map[string]struct{}, len(exp.Nodes))
		for j, nodeName := range exp.Nodes {
			if nodeName == "" {
				return fmt.Errorf("exports.prometheuses[%d].nodes[%d]: name is required", i, j)
			}
			if _, ok := nodeNames[nodeName]; !ok {
				return fmt.Errorf("exports.prometheuses[%d].nodes[%d]: unknown node %q", i, j, nodeName)
			}
			if _, dup := seenNodes[nodeName]; dup {
				return fmt.Errorf("exports.prometheuses[%d].nodes[%d]: duplicate node %q", i, j, nodeName)
			}
			seenNodes[nodeName] = struct{}{}
		}
	}
	return nil
}

// NormalizeListenPort accepts "9100" or ":9100" and returns ":9100".
func NormalizeListenPort(port string) (string, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("is required")
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	n, err := strconv.Atoi(port[1:])
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("%q is not a valid TCP port", port)
	}
	return port, nil
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
	return slices.Contains(n.Collects, kind)
}

// CollectStrings returns collects as plain strings for NodeState.
func (n NodeConfig) CollectStrings() []string {
	out := make([]string, len(n.Collects))
	for i, k := range n.Collects {
		out[i] = string(k)
	}
	return out
}
