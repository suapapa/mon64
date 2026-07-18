package domain

import "time"

// Snapshot is the canonical aggregate of all node states.
type Snapshot struct {
	CollectedAt time.Time   `json:"collected_at" yaml:"collected_at"`
	Nodes       []NodeState `json:"nodes" yaml:"nodes"`
}

// NodeState is the normalized per-node status regardless of collector type.
type NodeState struct {
	Name        string    `json:"name" yaml:"name"`
	CollectedAt time.Time `json:"collected_at" yaml:"collected_at"`
	Reachable   bool      `json:"reachable" yaml:"reachable"`
	LastError   string    `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	CPU         *float64  `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	GPU         *float64  `json:"gpu,omitempty" yaml:"gpu,omitempty"`
	MemUsed     *float64  `json:"mem_used,omitempty" yaml:"mem_used,omitempty"`
	MemCached   *float64  `json:"mem_cached,omitempty" yaml:"mem_cached,omitempty"`
	SwapUsed    *float64  `json:"swap_used,omitempty" yaml:"swap_used,omitempty"`
}

// ClampPercent restricts v to [0, 100].
func ClampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Ptr returns a pointer to v.
func Ptr(v float64) *float64 {
	return &v
}
