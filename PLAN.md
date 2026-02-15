# Plan: Improve Quality & Test Coverage for TPM-Go

## Context

TPM-Go currently has ~6 test files covering ~30% of packages (mostly the derived-metric processors). Critical infrastructure — state stores, sinks, the graph/pipeline orchestrators, and 2 of 6 processors — has zero tests. There are also several code quality issues: `log.Fatal`/`panic` in recoverable paths, missing config validation, no linters, and no graceful shutdown. This plan addresses both test coverage and code quality in priority order.

---

## Phase 1: Tooling & CI Improvements

### 1a. Add golangci-lint configuration
- Create `.golangci.yml` with sensible defaults (govet, errcheck, staticcheck, unused, gosimple, ineffassign)
- Add `lint` target to `Makefile`

### 1b. Add test coverage reporting
- Add `coverage` target to `Makefile`: `go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`
- Gives a baseline number to track progress

### 1c. Update CI workflow
- File: `.github/workflows/test.yml`
- Add linting step, race detector (`-race` flag), coverage reporting
- Fix Go version to match go.mod (1.22)

---

## Phase 2: Critical Code Quality Fixes

### 2a. Replace `log.Fatal` / `panic` with proper error handling
- `sources/http_source.go`: Replace `log.Fatal` on network/read errors with `slog.Error` + continue/retry
- `sinks/log_sink.go`: Replace `panic` on channel close / bad type assertion with `slog.Error` + continue
- `pipelines/pipeline.go`: Return errors from `NewPipeline` instead of silent nil / `os.Exit`

### 2b. Add config validation
- `config/config.go`: After unmarshaling, validate required fields (source URLs, intervals, pipeline types), validate that referenced IDs (sources, sinks, state stores) exist in config, remove dead duplicate `if err != nil` check

### 2c. Fix graceful shutdown
- `main.go`: Cancel context on signal receipt so goroutines can clean up
- Ensure `InMemoryStateStore.Close()` is called on shutdown

---

## Phase 3: Test Coverage (Tier 1 — Foundational)

### 3a. `state_stores/in_memory_store_test.go` (NEW)
- Test Get/Set/Delete basic operations
- Test TTL expiration (set short TTL, sleep, verify expired)
- Test `Get` with `new()` factory creating fresh messages
- Test concurrent read/write with `go test -race`
- Test `Close()` stops cleanup goroutine

### 3b. `processors/transit/feed_message_events/feed_message_event_processor_test.go` (NEW)
- Test valid GTFS-RT binary → FeedMessageEvent conversion
- Test malformed input handling
- Follow existing processor test pattern (channel-based with table-driven tests)

### 3c. `processors/transit/vehicle_position_events/vehicle_position_event_processor_test.go` (NEW)
- Test extraction of VehiclePositionEvents from FeedMessageEvent
- Test feed with multiple vehicles
- Test feed with no vehicles

---

## Phase 4: Test Coverage (Tier 2 — Sinks & Sources)

### 4a. `sinks/connector_sink_test.go` (NEW)
- Test that events written to ConnectorSink appear on the connector channel
- Test context cancellation stops the sink

### 4b. `sinks/log_sink_test.go` (NEW)
- Test that events are logged (capture slog output)
- Test graceful handling of channel closure

### 4c. `sources/connector_source_test.go` (NEW)
- Test that events put on connector channel are emitted by source
- Test context cancellation

### 4d. `event_server/event_server_test.go` (NEW)
- Test SSE client subscription and event delivery
- Test subscription filtering (agency_id, route_id)
- Test client disconnect cleanup

---

## Phase 5: Test Coverage (Tier 3 — Integration)

### 5a. `pipelines/pipeline_test.go` (NEW)
- Test wiring: mock source → processor → mock sink
- Test fan-out to multiple sinks
- Test pipeline with state store

### 5b. `graphs/graph_test.go` (NEW)
- Test graph construction from minimal valid config
- Test error on invalid config (missing references)

### 5c. Expand existing tests
- `sources/http_source_test.go`: Add error path tests (network failure, bad response)
- `config/config_test.go`: Add validation error tests
- All processor tests: Add edge case / error path subtests

---

## Verification

After each phase:
```bash
make lint              # Phase 1 — should pass clean
make test              # All phases — no regressions
make coverage          # Track coverage % increasing
go test -race ./...    # Phase 3+ — no race conditions
```

Target: go from ~25% to ~70%+ package-level test coverage.
