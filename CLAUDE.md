# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make generate        # Compile .proto files (required before build/test)
make test            # Run all tests (runs generate first)
make run             # Run the application (runs generate first)
go test -v ./processors/transit/stop_events/...  # Run a single package's tests
```

Requires: Go 1.22+, `protoc` with `protoc-gen-go` plugin.

## Architecture

TPM-Go is a real-time transit performance monitoring system that processes GTFS-RT vehicle position feeds into derived metrics (stop events, headways, dwell times, travel times).

### Core Abstraction: Pipeline-based Stream Processing

The system uses `github.com/reugn/go-streams` as its streaming foundation. All core interfaces (`Source`, `Sink`, `Processor`) wrap go-streams interfaces (`streams.Source`, `streams.Sink`, `streams.Flow`).

**Data flow is configured declaratively in YAML** (`config/configs/default.yaml`):

```
Graph (top-level orchestrator)
  └── Pipelines (each runs in its own goroutine)
        └── Source → Processor → FanOut → [Sink, Sink, ...]
```

Pipelines connect to each other via **Connectors** — in-memory channels where one pipeline's ConnectorSink writes to another pipeline's ConnectorSource.

### Processing Chain

```
HTTP Source (polls GTFS-RT feed)
  → FeedMessageProcessor (binary → FeedMessageEvent)
    → VehiclePositionEventProcessor (extracts positions)
      → StopEventProcessor (detects arrivals/departures using state)
        → HeadwayEventProcessor, DwellEventProcessor, TravelTimeEventProcessor
```

Each arrow represents a separate pipeline connected via connectors.

### Stateful Processors

`StopEventProcessor`, `HeadwayEventProcessor`, `DwellEventProcessor`, and `TravelTimeEventProcessor` use the `StateStore` interface to track previous events. The `InMemoryStateStore` implementation uses `sync.RWMutex` with TTL-based expiration.

### Key Directories

- `api/v1/events/` — Protobuf definitions and generated code (all event types)
- `config/` — YAML config parsing; each component type has its own config struct
- `graphs/` — Graph orchestrator: instantiates all components from config, runs pipelines
- `pipelines/` — Pipeline runner: wires Source → Processor → Sinks using go-streams
- `processors/transit/` — Six processor types, one subdirectory each
- `sources/` — Source implementations (HTTP polling is the primary one)
- `sinks/` — Sink implementations (console logging, SSE broadcasting, connectors)
- `state_stores/` — StateStore interface and implementations
- `event_server/` — SSE server for real-time event streaming to clients

### Configuration

Pipeline types in config: `feed_message`, `vehicle_position`, `stop_event`, `dwell_event`, `headway_event`, `travel_time`. Each pipeline references its sources, sinks, and state store by name.

### Data Model

All event types are defined in `api/v1/events/transit.proto`. Events share an `EventAttributes` message containing common fields (agency_id, vehicle_id, route_id, stop_id, trip_id, etc.). State stores use `proto.Message` as the value type.
