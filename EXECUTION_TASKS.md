# EXECUTION_TASKS.md

Task list for mon64 MVP. Status: `pending` | `in_progress` | `done` | `cancelled`.

Agents: pick the next `pending` task, set to `in_progress`, implement, verify (`gofmt`, `go test ./...`, `go vet ./...`, `go build ./...`), then mark `done`.

---

## T001 — Project bootstrap

**Status:** done

- [x] `go mod init`, directory skeleton
- [x] `.gitignore`, `README.md`, `Dockerfile`
- [x] `configs/example.yaml`

---

## T002 — Config package

**Status:** done

- [x] YAML load (`internal/config/config.go`)
- [x] Validation (listen, intervals, prom_fmt, collects, duplicate names)
- [x] Unit tests (`internal/config/config_test.go`)

---

## T003 — Domain model

**Status:** done

- [x] `NodeState`, `Snapshot` with optional `*float64` metrics
- [x] `ClampPercent` helper

---

## T004 — Metrics scraper

**Status:** done

- [x] HTTP fetch with timeout (`internal/collector/scraper.go`)
- [x] Non-2xx and network error handling

---

## T005 — Prometheus text parser

**Status:** done

- [x] `expfmt` integration (`internal/collector/parse.go`)
- [x] Labeled metric lookup helpers

---

## T006 — NodeExporterCollector

**Status:** done

- [x] CPU delta from `node_cpu_seconds_total`
- [x] Memory: MemTotal, MemAvailable, Cached
- [x] Swap with zero-total edge case
- [x] Tests with `ref/omv_*`, `ref/vraptor_*`

---

## T007 — NvMonitorCollector

**Status:** done

- [x] CPU from `nv_cpu_usage_percent{cpu="overall"}`
- [x] GPU average from `nv_gpu_utilization_percent`
- [x] Memory/swap from `nv_memory_*`, `nv_swap_*`
- [x] Tests with `ref/spark_nv-monitor.metrics`

---

## T008 — Collection engine

**Status:** done

- [x] Parallel per-node collection (`internal/collector/engine.go`)
- [x] Malformed metrics → unreachable state
- [x] Unreachable endpoint test

---

## T009 — Store & scheduler

**Status:** done

- [x] In-memory snapshot (`internal/store/store.go`)
- [x] Immediate collect on start + interval ticker
- [x] Thread-safe read API

---

## T010 — JSON/YAML exporters

**Status:** done

- [x] `internal/exporter/serialize.go`
- [x] Unit tests

---

## T011 — PNG badge renderer

**Status:** done

- [x] 128×128 PNG, dark theme, meters
- [x] Unreachable / n/a display
- [x] Document Tom Thumb BDF badge font (`ref/tom-thumb.bdf`)
- [x] Decode/size tests

---

## T012 — HTTP server

**Status:** done

- [x] Routes: `/`, `/api/v1/nodes`, `/api/v1/nodes.yaml`, `/api/v1/badge/{name}.png`, `/healthz`
- [x] Handler tests with httptest

---

## T013 — Web dashboard

**Status:** done

- [x] `web/index.html`, `web/static/style.css`
- [x] Embedded via `web/embed.go`
- [x] Responsive grid, badge thumbnails

---

## T014 — Main & graceful shutdown

**Status:** done

- [x] `cmd/mon64/main.go`
- [x] Signal handling, HTTP shutdown

---

## T015 — Agent documentation

**Status:** done

- [x] `AGENTS.md`
- [x] `EXECUTION_TASKS.md` (this file)

---

## T016 — CI verification pass

**Status:** done

- [x] Run `gofmt -w .` on all Go files
- [x] `go test ./...` green
- [x] `go vet ./...` clean
- [x] `go build ./cmd/mon64` succeeds
- [x] Fix any failures and re-run

---

## T017 — Tom Thumb BDF font for badges

**Status:** done

- [x] Parse/load `ref/tom-thumb.bdf` (embedded in `internal/exporter`)
- [x] Replace `basicfont` in `internal/exporter/png.go`
- [x] Keep embed in sync with `ref/tom-thumb.bdf` (test)
- Source: https://robey.lag.net/2010/01/23/tiny-monospace-font.html

---

## T018 — Integration test with mock Prometheus server (optional)

**Status:** pending

- [ ] httptest server serving fixture metrics
- [ ] End-to-end store → API flow test

---

## T019 — Production hardening (optional)

**Status:** done

- [x] Structured logging (`log/slog` JSON)
- [x] Request logging middleware (Gin)
- [x] Config hot-reload (`fsnotify` + `SIGHUP`)
- [x] mon64 self-metrics endpoint (`GET /metrics`)
- [x] HTTP server migrated to Gin

---

## T020 — Dummy metrics mode

**Status:** done

- [x] Add `-dummy` CLI flag
- [x] Require only node `name` and `collects` in dummy mode
- [x] Generate requested metrics without Prometheus endpoint requests
- [x] Add config and store tests

---

## Dependency graph

```
T001 → T002,T003 → T004,T005 → T006,T007 → T008 → T009 → T010,T011 → T012,T013 → T014 → T015 → T016
T017–T020 independent optional follow-ups
```
