package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/metrics"
	"github.com/suapapa/mon64/internal/server"
	"github.com/suapapa/mon64/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := &config.Config{
		Listen:         ":8080",
		ScrapeInterval: time.Hour,
		ScrapeTimeout:  time.Second,
		Nodes: []config.NodeConfig{{
			Name:     "spark",
			PromFmt:  config.PromFmtNvMonitor,
			Collects: []config.CollectKind{config.CollectCPU},
		}},
	}
	st := store.New(cfg)
	cpu := 1.0
	st.SetSnapshotForTest(domain.Snapshot{
		CollectedAt: time.Now().UTC(),
		Nodes: []domain.NodeState{{
			Name:      "spark",
			Reachable: true,
			CPU:       &cpu,
		}},
	})
	return st
}

func TestHandlers(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	reg := metrics.NewRegistry()
	srv, err := server.New(testStore(t), log, reg)
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Engine()

	t.Run("healthz", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
			t.Fatalf("healthz: %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("metrics", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "mon64_scrapes_total") {
			t.Fatalf("missing self-metrics: %s", body)
		}
	})

	t.Run("nodes json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Fatalf("content-type = %q", ct)
		}
	})

	t.Run("nodes yaml", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes.yaml", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("badge", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/badge/spark.png", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("content-type = %q", ct)
		}
	})

	t.Run("badge missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/badge/nope.png", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("events sse", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)

		done := make(chan struct{})
		go func() {
			h.ServeHTTP(rec, req)
			close(done)
		}()

		time.Sleep(20 * time.Millisecond)
		if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Fatalf("content-type = %q", ct)
		}
		if !strings.Contains(rec.Body.String(), ": connected") {
			t.Fatalf("missing connected comment: %q", rec.Body.String())
		}

		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("SSE handler did not exit")
		}
	})
}
