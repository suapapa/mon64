# AGENTS.md — mon64 agent guide

This document orients automated agents (and humans) working on the **mon64** repository.

## Project summary

**mon64** polls Prometheus text endpoints on remote servers, normalizes metrics into a common schema, stores the latest snapshot in memory, and serves:

- Web dashboard (`GET /`)
- JSON API (`GET /api/v1/nodes`) — **canonical**
- YAML API (`GET /api/v1/nodes.yaml`)
- 128×128 PNG badges (`GET /api/v1/badge/{name}.png`)
- Health check (`GET /healthz`)
- Self-metrics (`GET /metrics`)

Requirements source of truth: `doc/PLAN.md`. Metric fixtures: `ref/*.metrics`.

## Tech stack

| Layer | Choice |
|-------|--------|
| Language | Go 1.26+ (latest stable) |
| HTTP | Gin (`github.com/gin-gonic/gin`) on stdlib `http.Server` |
| Logging | `log/slog` (JSON) |
| Templates | `html/template` + embedded `web/` |
| Config | YAML (`gopkg.in/yaml.v3`) |
| Metrics parsing | `github.com/prometheus/common/expfmt` |
| Storage | In-memory snapshot only (no DB) |
| PNG | `image/png` + embedded Tom Thumb BDF (`ref/tom-thumb.bdf`) |

## Directory layout

```
cmd/mon64/           Entry point
internal/config/      YAML load + validation
internal/domain/      Normalized NodeState / Snapshot
internal/collector/   Scraper, node-exporter & nv-monitor collectors
internal/store/       Scheduler + in-memory snapshot
internal/exporter/    JSON, YAML, PNG renderers
internal/metrics/      Self-metrics registry + /metrics exposition
internal/server/      Gin HTTP routes + middleware
web/                  Embedded dashboard (index.html, static/)
configs/              Example YAML
ref/                  Metric fixtures & BDF font source (`tom-thumb.bdf`)
doc/                  PLAN, REFERENCE
```

## Key design decisions

1. **Optional metrics**: Uncollected or incalculable fields use `null`/omitted JSON fields (`*float64` nil), never fake `0`.
2. **node-exporter CPU**: Requires two scrapes; first scrape leaves CPU unavailable. Delta formula: `100 * (1 - idle_delta / total_delta)`.
3. **nv-monitor GPU**: Average of all `nv_gpu_utilization_percent` label values when multiple GPUs exist.
4. **Scrape model**: Background goroutine collects on startup + interval; HTTP handlers read store only.
5. **Failure isolation**: Per-node errors set `reachable: false` + `last_error`; other nodes and the server continue.
6. **Badge font**: Tom Thumb BDF (`ref/tom-thumb.bdf`), embedded and rendered at 2× in `internal/exporter`.
7. **Config hot-reload**: `internal/config/watcher.go` + `SIGHUP`; `listen` changes need process restart.
8. **HTTP**: Gin router; handlers read store snapshot only.

## Commands

```bash
# Format
gofmt -w .

# Test / vet / build
go test ./...
go vet ./...
go build -o bin/mon64 ./cmd/mon64

# Run locally
go run ./cmd/mon64 -config configs/example.yaml
```

## Configuration reference

See `configs/example.yaml`. Valid `prom_fmt`: `node-exporter`, `nv-monitor`. Valid `collects`: `cpu`, `gpu`, `mem`, `swap`. GPU is invalid for `node-exporter`.

## Testing conventions

- Use fixtures in `ref/` — do not invent metric names.
- Collector tests: `internal/collector/collector_test.go`
- Handler tests: `internal/server/server_test.go`
- Store test helper: `SetSnapshotForTest`

## Task tracking

Implementation tasks with IDs live in **`EXECUTION_TASKS.md`**. Update task status there when completing work.

## Do not

- Scrape remote endpoints on API request path
- Add databases or heavy frameworks without explicit approval
- Commit secrets or `.env` files
- Assume `ref/mon64_idea.png` exists (reference only)

## Remaining enhancements (optional)

- Configurable badge height / themes
- Authentication for admin endpoints
