package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Scraper fetches raw Prometheus text from an endpoint.
type Scraper struct {
	client *http.Client
}

// NewScraper builds a scraper with the given timeout.
func NewScraper(timeout time.Duration) *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: timeout},
	}
}

// Fetch retrieves metrics body from url. Caller must close the returned ReadCloser.
// When apiKey is non-empty, Authorization: Bearer {apiKey} is sent.
func (s *Scraper) Fetch(ctx context.Context, url, apiKey string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}
