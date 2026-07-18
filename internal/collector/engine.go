package collector

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

// Engine orchestrates scraping and collection for all configured nodes.
type Engine struct {
	mu      sync.RWMutex
	scraper *Scraper
	nodeExp *NodeExporterCollector
	nvMon   *NvMonitorCollector
}

// NewEngine creates a collection engine.
func NewEngine(timeout time.Duration) *Engine {
	return &Engine{
		scraper: NewScraper(timeout),
		nodeExp: NewNodeExporterCollector(),
		nvMon:   NewNvMonitorCollector(),
	}
}

// SetScrapeTimeout updates the HTTP client timeout for remote scrapes.
func (e *Engine) SetScrapeTimeout(d time.Duration) {
	e.mu.Lock()
	e.scraper = NewScraper(d)
	e.mu.Unlock()
}

// CollectOne scrapes a single node and returns its normalized state.
func (e *Engine) CollectOne(ctx context.Context, node config.NodeConfig) domain.NodeState {
	return e.collectOne(ctx, node)
}

// CollectAll scrapes every node in parallel and returns normalized states.
func (e *Engine) CollectAll(ctx context.Context, nodes []config.NodeConfig) []domain.NodeState {
	results := make([]domain.NodeState, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func(i int, node config.NodeConfig) {
			defer wg.Done()
			results[i] = e.collectOne(ctx, node)
		}(i, node)
	}
	wg.Wait()
	return results
}

func (e *Engine) collectOne(ctx context.Context, node config.NodeConfig) domain.NodeState {
	at := time.Now().UTC()
	e.mu.RLock()
	scraper := e.scraper
	e.mu.RUnlock()
	body, err := scraper.Fetch(ctx, node.PromEndpoint, node.PromAPIKey)
	if err != nil {
		return domain.NodeState{
			Name:        node.Name,
			CollectedAt: at,
			Reachable:   false,
			LastError:   err.Error(),
		}
	}
	defer body.Close()

	col, err := e.collectorFor(node.PromFmt)
	if err != nil {
		return domain.NodeState{
			Name:        node.Name,
			CollectedAt: at,
			Reachable:   false,
			LastError:   err.Error(),
		}
	}
	return col.Collect(ctx, node, body, at)
}

func (e *Engine) collectorFor(fmtName string) (Collector, error) {
	switch fmtName {
	case config.PromFmtNodeExporter:
		return e.nodeExp, nil
	case config.PromFmtNvMonitor:
		return e.nvMon, nil
	default:
		return nil, fmt.Errorf("unsupported prom_fmt %q", fmtName)
	}
}

// CollectFromReader is a test helper that skips HTTP and parses body directly.
func (e *Engine) CollectFromReader(node config.NodeConfig, body io.Reader, at time.Time) domain.NodeState {
	col, err := e.collectorFor(node.PromFmt)
	if err != nil {
		return domain.NodeState{
			Name:        node.Name,
			CollectedAt: at,
			Reachable:   false,
			LastError:   err.Error(),
		}
	}
	return col.Collect(context.Background(), node, body, at)
}
