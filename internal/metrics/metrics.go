package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Registry holds mon64 self-metrics for Prometheus exposition.
type Registry struct {
	httpMu   sync.Mutex
	httpReqs map[string]uint64
}

// NewRegistry creates an empty metrics registry.
func NewRegistry() *Registry {
	return &Registry{httpReqs: make(map[string]uint64)}
}

// ObserveHTTP records one HTTP request (route template, not raw URL).
func (r *Registry) ObserveHTTP(method, route, status string) {
	key := method + "|" + route + "|" + status
	r.httpMu.Lock()
	r.httpReqs[key]++
	r.httpMu.Unlock()
}

// ScrapeStats are collector-side counters updated after each scrape round.
type ScrapeStats struct {
	ScrapesTotal           uint64
	LastUnreachableNodes   uint64
	NodesConfigured        int
	NodesReachable         int
	LastScrapeUnix         float64
	LastScrapeDurationSecs float64
}

// WritePrometheus renders text exposition format.
func (r *Registry) WritePrometheus(w io.Writer, scrape ScrapeStats) {
	fmt.Fprintln(w, "# HELP mon64_scrapes_total Total scrape rounds completed.")
	fmt.Fprintln(w, "# TYPE mon64_scrapes_total counter")
	fmt.Fprintf(w, "mon64_scrapes_total %d\n", scrape.ScrapesTotal)

	fmt.Fprintln(w, "# HELP mon64_scrape_unreachable_nodes Unreachable nodes in the last scrape.")
	fmt.Fprintln(w, "# TYPE mon64_scrape_unreachable_nodes gauge")
	fmt.Fprintf(w, "mon64_scrape_unreachable_nodes %d\n", scrape.LastUnreachableNodes)

	fmt.Fprintln(w, "# HELP mon64_nodes_configured Configured node count.")
	fmt.Fprintln(w, "# TYPE mon64_nodes_configured gauge")
	fmt.Fprintf(w, "mon64_nodes_configured %d\n", scrape.NodesConfigured)

	fmt.Fprintln(w, "# HELP mon64_nodes_reachable Reachable nodes in the last scrape.")
	fmt.Fprintln(w, "# TYPE mon64_nodes_reachable gauge")
	fmt.Fprintf(w, "mon64_nodes_reachable %d\n", scrape.NodesReachable)

	fmt.Fprintln(w, "# HELP mon64_last_scrape_timestamp_seconds Unix time of the last scrape round.")
	fmt.Fprintln(w, "# TYPE mon64_last_scrape_timestamp_seconds gauge")
	fmt.Fprintf(w, "mon64_last_scrape_timestamp_seconds %.3f\n", scrape.LastScrapeUnix)

	fmt.Fprintln(w, "# HELP mon64_last_scrape_duration_seconds Duration of the last scrape round.")
	fmt.Fprintln(w, "# TYPE mon64_last_scrape_duration_seconds gauge")
	fmt.Fprintf(w, "mon64_last_scrape_duration_seconds %.6f\n", scrape.LastScrapeDurationSecs)

	fmt.Fprintln(w, "# HELP mon64_http_requests_total HTTP requests served.")
	fmt.Fprintln(w, "# TYPE mon64_http_requests_total counter")

	r.httpMu.Lock()
	keys := make([]string, 0, len(r.httpReqs))
	for k := range r.httpReqs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) != 3 {
			continue
		}
		fmt.Fprintf(w, "mon64_http_requests_total{method=%q,path=%q,status=%q} %d\n",
			parts[0], parts[1], parts[2], r.httpReqs[key])
	}
	r.httpMu.Unlock()
}

// Global is the process-wide metrics registry.
var Global = NewRegistry()
