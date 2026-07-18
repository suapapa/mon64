package collector_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/collector"
)

func TestScraperFetchAuthorization(t *testing.T) {
	const wantKey = "secret-token"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# metrics\n"))
	}))
	defer srv.Close()

	scraper := collector.NewScraper(time.Second)

	body, err := scraper.Fetch(t.Context(), srv.URL, wantKey)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	body.Close()

	if gotAuth != "Bearer "+wantKey {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer "+wantKey)
	}
}

func TestScraperFetchNoAuthorizationWhenKeyEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# metrics\n"))
	}))
	defer srv.Close()

	scraper := collector.NewScraper(time.Second)

	body, err := scraper.Fetch(t.Context(), srv.URL, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	body.Close()

	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}
