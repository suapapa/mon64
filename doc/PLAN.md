현재 작업 디렉터리의 `mon64` 프로젝트를 MVP 수준으로 스캐폴딩하고 구현해 줘.

먼저 다음 파일을 읽고 실제 요구사항과 Prometheus 메트릭 이름을 확인해:
- `doc/PLAN.md`
- `doc/REFERENCE.md`
- `ref/spark_nv-monitor.metrics`
- `ref/omv_node-exporter_metrics.metrics`
- `ref/vraptor_node-exporter_metrics.metrics`
- `ref/tom-thumb.bdf` (Tom Thumb; see also `ref/cherry-10-r.bdf`)

프로젝트 목적:
여러 원격 서버가 노출하는 Prometheus endpoint를 주기적으로 수집하고, 정규화된 서버 상태를 Web UI, JSON/YAML API, 128×128 PNG 이미지로 제공한다.

기술 스택:
- Go 최신 안정 버전
- 단일 실행 파일
- HTTP 서버는 Go 표준 라이브러리를 우선 사용
- Frontend는 `html/template`과 최소한의 CSS/JS 사용
- 과도한 프레임워크나 데이터베이스는 사용하지 않음
- 필요한 경우 검증된 Prometheus text parser 라이브러리는 사용 가능

먼저 간단한 구현 계획을 제시한 뒤 바로 작업해. 불명확한 세부사항은 아래 기본 결정을 적용하고, 구현을 멈추고 질문하지 마.

핵심 구조:
- Config: YAML 설정 로딩 및 검증
- Scraper: endpoint HTTP 요청, timeout 처리
- Collector:
  - `NodeExporterCollector`
  - `NvMonitorCollector`
- Domain model: collector 종류와 무관한 정규화된 상태
- Scheduler/Store: 주기적으로 병렬 수집하고 최신 snapshot을 메모리에 보관
- Exporter/Renderer:
  - JSON
  - YAML
  - 128×128 PNG
- HTTP handler
- Web frontend

권장 디렉터리 구조:
- `cmd/mon64/`
- `internal/config/`
- `internal/domain/`
- `internal/collector/`
- `internal/store/`
- `internal/badge/`
- `internal/export/`
- `internal/server/`
- `web/`
- `configs/`
- `README.md`

설정 형식:
```yaml
listen: ":8080"
scrape_interval: 15s
scrape_timeout: 5s

nodes:
  - name: spark
    prom_fmt: nv-monitor
    prom_endpoint: http://spark.local:9101
    collects:
      - cpu
      - gpu
      - mem
      - swap

  - name: vraptor
    prom_fmt: node-exporter
    prom_endpoint: http://vraptor.local:9100
    collects:
      - cpu
      - mem

  - name: omv
    prom_fmt: node-exporter
    prom_endpoint: http://omv.local:9100
    collects:
      - cpu
      - mem
```

---

정규화 모델에는 최소한 다음 정보가 포함되어야 해:
- node name
- collected timestamp
- endpoint 상태/reachable 여부
- 마지막 오류
- CPU usage percent
- GPU usage percent
- memory used percent
- memory cached percent
- swap used percent

수집하지 않거나 계산할 수 없는 값은 0으로 위장하지 말고 optional/null로 표현해.

계산 규칙:
- 모든 퍼센트는 0~100 범위로 clamp
- node-exporter CPU:
  - `node_cpu_seconds_total`의 두 수집 시점 사이 delta를 사용
  - 전체 CPU 기준으로 `100 * (1 - idle_delta / total_delta)`
  - 첫 수집처럼 이전 표본이 없으면 CPU 값을 unavailable로 둠
- node-exporter memory:
  - used = `(MemTotal - MemAvailable) / MemTotal * 100`
  - cached = `Cached / MemTotal * 100`
- node-exporter swap:
  - used = `(SwapTotal - SwapFree) / SwapTotal * 100`
  - SwapTotal이 0이면 unavailable
- nv-monitor:
  - fixture에 실제 존재하는 `nv_cpu_usage_percent`,
    `nv_gpu_utilization_percent`,
    `nv_memory_total_bytes`,
    `nv_memory_used_bytes`,
    `nv_memory_bufcache_bytes`,
    `nv_swap_total_bytes`,
    `nv_swap_used_bytes`를 사용
  - 여러 GPU가 있으면 평균 utilization을 사용하고 이 정책을 문서화

수집 동작:
- 서버 시작 직후 한 번 수집하고 이후 interval마다 수집
- 노드별 요청은 병렬 실행
- API 요청 때마다 원격 endpoint를 직접 scrape하지 말고 저장된 최신 snapshot을 반환
- 한 노드의 실패가 다른 노드 수집이나 HTTP 서버를 중단시키지 않도록 함
- timeout, non-2xx 응답, malformed metrics를 명시적으로 처리
- 종료 signal을 받아 graceful shutdown

HTTP endpoint:
- `GET /` : 모든 노드의 상태를 보여주는 간단한 반응형 대시보드
- `GET /api/v1/nodes` : JSON snapshot
- `GET /api/v1/nodes.yaml` : YAML snapshot
- `GET /api/v1/badge/{node_name}.png` : 해당 노드의 128×H PNG (너비 128 고정)
- `GET /healthz` : 프로세스 상태 확인

> `/api/v1/nodes`를 canonical endpoint로 사용해.

128×H PNG:
- 반드시 정확히 128×H 크기와 `image/png` content type
- 어두운 배경
- 노드 이름
- CPU/GPU/MEM/SWAP 값을 작은 horizontal bar나 meter로 표현
- 정상/경고/위험 상태를 구분하는 일관된 색상 테마
- unavailable 값과 unreachable 상태를 분명히 표시
- 배지 글꼴은 Tom Thumb BDF (`ref/tom-thumb.bdf`)를 파싱·임베드해 사용 (128×128에서는 2× 스케일)
- PLAN에 언급된 참고 이미지는 현재 실제 파일이 없을 수 있으므로 존재한다고 가정하지 말 것
- 참고 이미지: `ref/mon64_idea.png`

테스트:
- 제공된 세 metrics fixture를 직접 사용하는 collector 테스트
- node-exporter CPU delta 계산 테스트
- memory/swap 계산과 zero-total edge case 테스트
- nv-monitor parsing 테스트
- 설정 검증 테스트
- JSON/YAML handler 테스트
- PNG가 128×128이고 decode 가능한지 확인하는 테스트
- unreachable 및 malformed endpoint 동작 테스트

산출물:
- 빌드 가능한 Go 프로젝트
- 예제 설정 파일
- 실행 방법과 endpoint 설명이 포함된 README
- `.gitignore`
- 가능하면 multi-stage Dockerfile
- 핵심 코드에만 간결한 주석
- 실제 fixture와 맞지 않는 가상의 metric 이름을 만들지 말 것

마지막으로 다음을 실행해 검증해:
- formatter
- `go test ./...`
- `go vet ./...`
- `go build ./...`

실패하면 원인을 고치고 다시 검증해. 완료 보고에는 생성한 구조, 주요 설계 결정, 테스트 결과, 남은 TODO만 간결하게 정리
