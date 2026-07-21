package prometheus_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/export/prometheus"
)

type snapSrc struct {
	snap domain.Snapshot
}

func (s snapSrc) Snapshot() domain.Snapshot { return s.snap }

func TestWritePrometheusFiltersAndOmitsNil(t *testing.T) {
	snap := domain.Snapshot{
		Nodes: []domain.NodeState{
			{
				Name:        "spark",
				Reachable:   true,
				CollectedAt: time.Unix(1_700_000_000, 0).UTC(),
				CPU:         domain.Ptr(12.5),
				GPU:         domain.Ptr(80),
				MemUsed:     domain.Ptr(40),
			},
			{
				Name:      "omv",
				Reachable: false,
				LastError: "timeout",
			},
			{
				Name:      "vraptor",
				Reachable: true,
				CPU:       domain.Ptr(1),
			},
		},
	}

	var buf bytes.Buffer
	prometheus.WritePrometheus(&buf, snap, []string{"spark", "omv", "missing"})
	out := buf.String()

	for _, want := range []string{
		`mon64_node_reachable{node="spark"} 1`,
		`mon64_node_reachable{node="omv"} 0`,
		`mon64_node_cpu_percent{node="spark"} 12.5`,
		`mon64_node_gpu_percent{node="spark"} 80`,
		`mon64_node_mem_used_percent{node="spark"} 40`,
		`mon64_node_collected_timestamp_seconds{node="spark"} 1700000000`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, refuse := range []string{
		`node="vraptor"`,
		`node="missing"`,
		`mon64_node_cpu_percent{node="omv"}`,
		`mon64_node_swap_used_percent`,
		`mon64_node_mem_cached_percent`,
	} {
		if strings.Contains(out, refuse) {
			t.Fatalf("unexpected %q in:\n%s", refuse, out)
		}
	}
}

func TestExporterServesMetrics(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	src := snapSrc{snap: domain.Snapshot{
		Nodes: []domain.NodeState{{
			Name:      "spark",
			Reachable: true,
			CPU:       domain.Ptr(9),
		}},
	}}
	exp, err := prometheus.New(src, []config.PrometheusExport{{
		Port:  ":" + strconv.Itoa(port),
		Nodes: []string{"spark"},
	}}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		exp.Run(ctx)
		close(done)
	}()

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/metrics"
	var body string
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body = string(b)
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET /metrics failed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(body, `mon64_node_cpu_percent{node="spark"} 9`) {
		t.Fatalf("body = %s", body)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("exporter did not stop")
	}
}
