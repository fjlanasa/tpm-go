# TPM-Go

A real-time transit performance monitoring system that processes GTFS-RT vehicle position feeds into derived metrics: stop events, headways, dwell times, and travel times.

The system polls live transit feeds (currently MBTA), detects when vehicles arrive at and depart from stops using a state machine, then derives secondary metrics from those events. All processing is defined declaratively in YAML and runs as a graph of concurrent pipelines connected by channels or Redis pub/sub.

## Architecture

```
HTTP Source (polls GTFS-RT feed)
  → FeedMessageProcessor (binary protobuf → FeedMessageEvent)
    → VehiclePositionProcessor (extracts per-vehicle positions)
      → StopEventProcessor (state machine: detects arrivals/departures)
        → HeadwayEventProcessor   (time between consecutive trips at a stop)
        → DwellEventProcessor     (time a vehicle spends stopped)
        → TravelTimeProcessor     (time between consecutive stops)
```

Each arrow is a separate pipeline running in its own goroutine. Pipelines connect via **connectors** — in-memory channels (single process) or Redis pub/sub (distributed). The same codebase supports both topologies through configuration alone.

```
┌─────────────────────────────────────────────────────────────┐
│  Graph (top-level orchestrator)                             │
│                                                             │
│  ┌──────────┐   connector   ┌──────────┐   connector       │
│  │ Pipeline  │──────────────▶│ Pipeline  │──────────────▶...│
│  │ Source →  │               │ Source →  │                   │
│  │ Processor │               │ Processor │                   │
│  │ → FanOut  │               │ → FanOut  │                   │
│  │   → Sinks │               │   → Sinks │                   │
│  └──────────┘               └──────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

Sinks include: console logging, SSE (real-time browser streaming), PostgreSQL, S3/Parquet, and Redis pub/sub.

### Data Model

All events are defined in Protocol Buffers (`api/v1/events/transit.proto`) and share a common `EventAttributes` message (agency, route, stop, trip, vehicle, timestamp). This gives type-safe serialization across the pipeline and into storage.

## Design Decisions

**Why a pipeline graph with connectors?** Each processing stage has different throughput characteristics and failure modes. Pipeline isolation means a slow Postgres sink doesn't back-pressure the feed poller. Connectors as an abstraction let the same code run as a monolith (in-memory channels) or distributed (Redis pub/sub) without changing processor logic.

**Why a state machine for stop event detection?** GTFS-RT provides point-in-time vehicle snapshots, not discrete events. Detecting arrivals and departures requires tracking each vehicle's previous status and comparing it to the current one. The state machine in `StopEventProcessor` handles edge cases like skipped stops (vehicle jumps from `STOPPED_AT` one stop to `STOPPED_AT` another) by synthesizing the missing departure and arrival events.

**Why Protobuf?** The input data is already GTFS-RT protobuf. Using protobuf for internal events too means a single serialization format across the system — no format conversion overhead, and the state store can serialize/deserialize events for Redis without a separate encoding layer.

**Why `go-streams`?** It provides the minimal streaming primitives (Source, Flow, Sink) without the weight of a full framework. The library handles backpressure through Go channels and composes cleanly. The custom `FanOut` sink was the only extension needed to support multiple downstream consumers.

**Why Go?** Goroutines map naturally to the concurrent pipeline model. Each pipeline, each source poll, and each sink flush runs in its own goroutine with context-based cancellation. The runtime handles scheduling without thread pool configuration.

## Quick Start

### Docker Compose (recommended)

**All-in-one** — runs all 6 pipelines in a single container with Postgres, MinIO (S3), and Grafana:

```bash
docker compose --profile all-in-one up --build
```

- SSE event stream: http://localhost:8080/events
- Grafana (logs): http://localhost:3000
- MinIO console: http://localhost:9001

**Distributed** — splits ingestion and processing across two containers connected via Redis:

```bash
docker compose --profile distributed up --build
```

- Ingestion SSE (vehicle positions): http://localhost:8080/events
- Processing SSE (derived events): http://localhost:8081/events

### Local Development

Prerequisites: Go 1.22+, `protoc` with `protoc-gen-go`, `golangci-lint`

```bash
make setup           # Install protoc-gen-go plugin and download Go modules
make generate        # Compile .proto files
make test            # Run all tests
make lint            # Run linter
make run             # Run with default config (console + SSE sinks only)
```

### Verifying Output

```bash
# Stream events in your terminal
curl -N http://localhost:8080/events

# Check Postgres (all-in-one or distributed profile)
docker compose exec postgres psql -U tpm -c "SELECT count(*) FROM stop_events"

# Check S3/Parquet files
docker compose exec minio-init mc ls local/tpm-events/transit/ --recursive
```

## Configuration

Pipelines are declared in YAML. Each pipeline specifies its type, sources, sinks, and state store. Three configurations are included:

| Config | Use case | Connectors |
|--------|----------|------------|
| `default.yaml` | Local dev — console + SSE output only | In-memory channels |
| `all-in-one.yaml` | Full deployment — adds Postgres + Parquet/S3 sinks | In-memory channels |
| `distributed-*.yaml` | Horizontal scaling — ingestion and processing containers | Redis pub/sub |

Adding a new transit agency is a config change — add an HTTP source pointing to the agency's GTFS-RT feed URL and a corresponding `feed_message` pipeline.

## Testing

All tests run in-process with no external dependencies:

- **Redis**: [`alicebob/miniredis`](https://github.com/alicebob/miniredis) provides a real Redis protocol implementation in-memory
- **Postgres**: [`DATA-DOG/go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock) for sink tests

```bash
make test            # Run all tests
make coverage        # Run tests with coverage report
```

## Project Structure

```
api/v1/events/       Protobuf definitions and generated code
config/              YAML config parsing and validation
graphs/              Graph orchestrator — builds pipelines from config
pipelines/           Pipeline runner — wires Source → Processor → Sinks
processors/transit/  Six processor types (one subdirectory each)
sources/             HTTP polling, Redis pub/sub, connectors
sinks/               Console, SSE, Postgres, S3/Parquet, Redis, connectors
state_stores/        StateStore interface with in-memory and Redis implementations
event_server/        SSE server for real-time event streaming
```
