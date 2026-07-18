package metrics_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/suapapa/mon64/internal/metrics"
)

func TestWritePrometheus(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.ObserveHTTP("GET", "/healthz", "200")
	var buf bytes.Buffer
	reg.WritePrometheus(&buf, metrics.ScrapeStats{
		ScrapesTotal:           3,
		LastUnreachableNodes:   1,
		NodesConfigured:        2,
		NodesReachable:         1,
		LastScrapeUnix:         1_700_000_000,
		LastScrapeDurationSecs: 0.42,
	})
	out := buf.String()
	for _, want := range []string{
		"mon64_scrapes_total 3",
		"mon64_nodes_configured 2",
		`mon64_http_requests_total{method="GET",path="/healthz",status="200"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
