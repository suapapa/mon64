package store

import (
	"context"
	"sync"
	"time"

	"github.com/suapapa/mon64/internal/collector"
	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/metrics"
)

type subscriber struct {
	ch chan struct{}
}

// Store keeps the latest snapshot in memory and runs periodic collection.
type Store struct {
	mu       sync.RWMutex
	snap     domain.Snapshot
	engine   *collector.Engine
	nodes    []config.NodeConfig
	interval time.Duration
	dummy    bool
	reloadCh chan struct{}

	workerCancel context.CancelFunc

	subsMu sync.Mutex
	subs   map[uint64]*subscriber
	subSeq uint64

	scrapesTotal    uint64
	lastUnreachable uint64
	lastScrapeDur   time.Duration
	lastScrapeAt    time.Time
}

// New creates a store bound to configuration.
func New(cfg *config.Config) *Store {
	return newStore(cfg, false)
}

// NewDummy creates a store that generates metrics without scraping endpoints.
func NewDummy(cfg *config.Config) *Store {
	return newStore(cfg, true)
}

func newStore(cfg *config.Config, dummy bool) *Store {
	return &Store{
		engine:   collector.NewEngine(cfg.ScrapeTimeout),
		nodes:    append([]config.NodeConfig(nil), cfg.Nodes...),
		interval: cfg.ScrapeInterval,
		dummy:    dummy,
		reloadCh: make(chan struct{}, 1),
		subs:     make(map[uint64]*subscriber),
	}
}

// Reload applies a new configuration without restarting the process.
// Listen address changes are ignored (requires restart).
func (s *Store) Reload(cfg *config.Config) error {
	var err error
	if s.dummy {
		err = cfg.ValidateDummy()
	} else {
		err = cfg.Validate()
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.nodes = append([]config.NodeConfig(nil), cfg.Nodes...)
	s.interval = cfg.ScrapeInterval
	s.engine.SetScrapeTimeout(cfg.ScrapeTimeout)
	s.syncSnapshotNodesLocked(cfg.Nodes)
	s.mu.Unlock()

	select {
	case s.reloadCh <- struct{}{}:
	default:
	}
	s.notify()
	return nil
}

// Subscribe returns a channel notified on snapshot changes and an unsubscribe func.
func (s *Store) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.subsMu.Lock()
	id := s.subSeq
	s.subSeq++
	s.subs[id] = &subscriber{ch: ch}
	s.subsMu.Unlock()

	unsubscribe := func() {
		s.subsMu.Lock()
		delete(s.subs, id)
		s.subsMu.Unlock()
	}
	return ch, unsubscribe
}

func (s *Store) notify() {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for _, sub := range s.subs {
		select {
		case sub.ch <- struct{}{}:
		default:
		}
	}
}

// Start runs per-node collectors until ctx is cancelled.
func (s *Store) Start(ctx context.Context) {
	s.runWorkers(ctx)
	for {
		select {
		case <-ctx.Done():
			s.stopWorkers()
			return
		case <-s.reloadCh:
			s.runWorkers(ctx)
			s.notify()
		}
	}
}

func (s *Store) stopWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workerCancel != nil {
		s.workerCancel()
		s.workerCancel = nil
	}
}

func (s *Store) runWorkers(ctx context.Context) {
	s.stopWorkers()

	workerCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.workerCancel = cancel
	nodes := append([]config.NodeConfig(nil), s.nodes...)
	defaultInterval := s.interval
	s.mu.Unlock()

	for _, node := range nodes {
		go s.nodeLoop(workerCtx, node, defaultInterval)
	}
}

func (s *Store) nodeLoop(ctx context.Context, node config.NodeConfig, defaultInterval time.Duration) {
	interval := node.EffectiveScrapeInterval(defaultInterval)
	s.collectNode(ctx, node)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectNode(ctx, node)
		}
	}
}

func (s *Store) collectNode(ctx context.Context, node config.NodeConfig) {
	start := time.Now()
	state := s.collectNodeState(ctx, node)

	s.mu.Lock()
	s.applyNodeStateLocked(state)
	s.scrapesTotal++
	s.lastScrapeDur = time.Since(start)
	s.lastScrapeAt = state.CollectedAt
	var unreachable int
	for _, n := range s.snap.Nodes {
		if !n.Reachable {
			unreachable++
		}
	}
	s.lastUnreachable = uint64(unreachable)
	s.mu.Unlock()

	s.notify()
}

func (s *Store) collectNodeState(ctx context.Context, node config.NodeConfig) domain.NodeState {
	if !s.dummy {
		return s.engine.CollectOne(ctx, node)
	}

	state := domain.NodeState{
		Name:        node.Name,
		CollectedAt: time.Now().UTC(),
		Reachable:   true,
	}
	if node.Wants(config.CollectCPU) {
		state.CPU = domain.Ptr(42)
	}
	if node.Wants(config.CollectGPU) {
		state.GPU = domain.Ptr(67)
	}
	if node.Wants(config.CollectMem) {
		state.MemUsed = domain.Ptr(58)
		state.MemCached = domain.Ptr(23)
	}
	if node.Wants(config.CollectSwap) {
		state.SwapUsed = domain.Ptr(12)
	}
	return state
}

func (s *Store) applyNodeStateLocked(state domain.NodeState) {
	for i, n := range s.snap.Nodes {
		if n.Name == state.Name {
			s.snap.Nodes[i] = state
			if state.CollectedAt.After(s.snap.CollectedAt) {
				s.snap.CollectedAt = state.CollectedAt
			}
			return
		}
	}
	s.snap.Nodes = append(s.snap.Nodes, state)
	if state.CollectedAt.After(s.snap.CollectedAt) {
		s.snap.CollectedAt = state.CollectedAt
	}
}

func (s *Store) syncSnapshotNodesLocked(nodes []config.NodeConfig) {
	byName := make(map[string]domain.NodeState, len(s.snap.Nodes))
	for _, n := range s.snap.Nodes {
		byName[n.Name] = n
	}
	ordered := make([]domain.NodeState, 0, len(nodes))
	for _, nc := range nodes {
		if st, ok := byName[nc.Name]; ok {
			ordered = append(ordered, st)
		}
	}
	s.snap.Nodes = ordered
}

// Snapshot returns a copy of the latest aggregate state.
func (s *Store) Snapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]domain.NodeState, len(s.snap.Nodes))
	copy(nodes, s.snap.Nodes)
	return domain.Snapshot{
		CollectedAt: s.snap.CollectedAt,
		Nodes:       nodes,
	}
}

// ScrapeStats returns collector metrics for /metrics exposition.
func (s *Store) ScrapeStats() metrics.ScrapeStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var reachable int
	for _, n := range s.snap.Nodes {
		if n.Reachable {
			reachable++
		}
	}
	var lastUnix float64
	if !s.lastScrapeAt.IsZero() {
		lastUnix = float64(s.lastScrapeAt.UnixNano()) / 1e9
	}
	return metrics.ScrapeStats{
		ScrapesTotal:           s.scrapesTotal,
		LastUnreachableNodes:   s.lastUnreachable,
		NodesConfigured:        len(s.nodes),
		NodesReachable:         reachable,
		LastScrapeUnix:         lastUnix,
		LastScrapeDurationSecs: s.lastScrapeDur.Seconds(),
	}
}

// NodeByName returns one node state or false if unknown.
func (s *Store) NodeByName(name string) (domain.NodeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, n := range s.snap.Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return domain.NodeState{}, false
}

// SetSnapshotForTest injects snapshot data in tests.
func (s *Store) SetSnapshotForTest(snap domain.Snapshot) {
	s.mu.Lock()
	s.snap = snap
	s.mu.Unlock()
}
