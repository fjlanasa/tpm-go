# Metric Processor Flows

This document describes the data flow through each processor in the TPM-Go pipeline, from raw GTFS-RT feed to derived transit metrics.

## Overview

The system is built on a declarative, pipeline-based streaming architecture using [go-streams](https://github.com/reugn/go-streams). Pipelines are configured in `config/configs/default.yaml` and run concurrently as goroutines. They communicate via **connectors** — named in-memory channels where one pipeline's `ConnectorSink` writes and another pipeline's `ConnectorSource` reads.

The full processing chain:

```
HTTP Source (polls GTFS-RT every 1s)
  └─ FeedMessageProcessor          [mbta-feed-message-pipeline]
       └─ VehiclePositionEventProcessor  [vp-pipeline]
            └─ StopEventProcessor        [se-pipeline]
                 ├─ HeadwayEventProcessor     [headway-pipeline]
                 ├─ DwellEventProcessor       [dwell-pipeline]
                 └─ TravelTimeEventProcessor  [travel-time-pipeline]
```

Each arrow represents a connector channel between pipelines. All processors also fan out to `console-sink` and `sse-sink` for logging and real-time SSE broadcasting.

---

## 1. FeedMessageProcessor

**Source:** `processors/transit/feed_message_events/feed_message_event_processor.go`
**Pipeline type:** `feed_message`
**Stateful:** No

### Input

Raw bytes (`[]uint8`) or a pre-decoded `*gtfs.FeedMessage` from the HTTP source, which polls the GTFS-RT vehicle positions endpoint (e.g., `https://cdn.mbta.com/realtime/VehiclePositions.pb`) once per second.

### Processing

1. Type-switches on the incoming value.
2. If raw bytes, deserializes using `proto.Unmarshal` into a `*gtfs.FeedMessage`. If deserialization fails, the message is dropped.
3. Wraps the decoded feed with the configured agency ID.

### Output

`*pb.FeedMessageEvent`

```proto
message FeedMessageEvent {
  string agency_id = 1;
  transit_realtime.FeedMessage feed_message = 2;
}
```

### Connector

Writes to `feed-message-connector` → consumed by `vp-pipeline`.

---

## 2. VehiclePositionEventProcessor

**Source:** `processors/transit/vehicle_position_events/vehicle_position_event_processor.go`
**Pipeline type:** `vehicle_position`
**Stateful:** No

### Input

`*pb.FeedMessageEvent` from `feed-message-connector`.

### Processing

Iterates over every entity in the `FeedMessage`. For each entity with a vehicle:

1. Maps the GTFS `CurrentStatus` enum to the internal `StopStatus` enum:
   - `IN_TRANSIT_TO` → `pb.StopStatus_IN_TRANSIT_TO`
   - `STOPPED_AT` → `pb.StopStatus_STOPPED_AT`
   - `INCOMING_AT` → `pb.StopStatus_INCOMING_AT`
2. Resolves vehicle ID: uses `vehicle.id` if present, falls back to `trip.trip_id`.
3. Converts the GTFS Unix timestamp to a `google.protobuf.Timestamp`.
4. Emits one `VehiclePositionEvent` per vehicle entity.

### Output

`*pb.VehiclePositionEvent` (one per vehicle entity in the feed)

```proto
message VehiclePositionEvent {
  EventAttributes attributes = 1;
  double latitude = 2;
  double longitude = 3;
}
```

`EventAttributes` fields populated: `agency_id`, `vehicle_id`, `route_id`, `stop_id`, `trip_id`, `service_date`, `direction_id`, `stop_sequence`, `stop_status`, `timestamp`.

### Connectors

Writes to `vp-connector` → consumed by `se-pipeline`. Also fans out to `console-sink` and `sse-sink`.

---

## 3. StopEventProcessor

**Source:** `processors/transit/stop_events/stop_event.go`
**Pipeline type:** `stop_event`
**Stateful:** Yes

### Input

`*pb.VehiclePositionEvent` from `vp-connector`.

### State

- **Store:** `InMemoryStateStore` with a 2-hour TTL (default; 1-hour TTL when storing each event).
- **Key:** `vehicle_id` (string)
- **Value:** The previous `*pb.VehiclePositionEvent` for that vehicle.

### Processing

This processor implements a **state machine** over each vehicle's `StopStatus` transitions. On each event:

1. Retrieve the previous state for the vehicle.
2. Store the current event as the new state (1-hour TTL).
3. If no previous state exists (first event for this vehicle), return — no output.
4. If `stop_status` and `stop_id` are unchanged, skip — no output.
5. Apply transition logic:

| Previous status | Current status | Previous stop_id == Current stop_id | Output |
|---|---|---|---|
| `IN_TRANSIT_TO` or `INCOMING_AT` | `STOPPED_AT` | — | ARRIVAL at current stop |
| `STOPPED_AT` | `IN_TRANSIT_TO` | — | DEPARTURE at previous stop |
| `IN_TRANSIT_TO` | `IN_TRANSIT_TO` | No | ARRIVAL at previous stop + DEPARTURE at previous stop |
| same | same | same | (no output) |

The third row handles the case where a vehicle skips the `STOPPED_AT` status entirely and is already in transit to the next stop — inferred arrival and departure are both emitted using the previous and current events respectively.

Fields `vehicle_id` and `stop_id` must both be non-empty or the event is discarded.

### Output

`*pb.StopEvent` — zero, one, or two events per input.

```proto
message StopEvent {
  EventAttributes attributes = 1;
  enum EventType { UNKNOWN = 0; ARRIVAL = 1; DEPARTURE = 2; }
  EventType stop_event_type = 14;
}
```

### Connectors

Writes to `headway-connector`, `dwell-connector`, and `travel-time-connector`. Also fans out to `console-sink` and `sse-sink`.

---

## 4. HeadwayEventProcessor

**Source:** `processors/transit/headway_events/headway_event_processor.go`
**Pipeline type:** `headway_event`
**Stateful:** Yes

### Input

`*pb.StopEvent` from `headway-connector`. Only `DEPARTURE` events are processed; arrivals are discarded immediately.

### State

- **Store:** `InMemoryStateStore` with a 2-hour TTL (1-hour TTL per entry).
- **Key:** `"routeID-directionID-stopID"` — identifies a directional stop on a route.
- **Value:** The previous `DEPARTURE` `*pb.StopEvent` at that stop.

### Processing

Headway measures the **time gap between consecutive vehicle departures** at the same directional stop on the same route.

1. Build key from `route_id`, `direction_id`, `stop_id`.
2. Look up the previous departure for this key.
3. If none found (first departure), store the current event and return — no output.
4. If the previous departure is from the **same vehicle**, skip — avoids false headways from vehicle position loops.
5. Compute: `headway_seconds = current_timestamp − previous_timestamp`.
6. Update state with the current event.
7. Emit a `HeadwayTimeEvent`.

### Output

`*pb.HeadwayTimeEvent` — zero or one event per DEPARTURE input.

```proto
message HeadwayTimeEvent {
  EventAttributes attributes = 1;
  string leading_vehicle_id = 2;
  string following_vehicle_id = 3;
  int32 headway_seconds = 4;
}
```

`leading_vehicle_id` is the vehicle that departed previously; `following_vehicle_id` is the current vehicle.

### Connectors

Terminal pipeline. Writes to `console-sink` and `sse-sink` only.

---

## 5. DwellEventProcessor

**Source:** `processors/transit/dwell_events/dwell_event_processor.go`
**Pipeline type:** `dwell_event`
**Stateful:** Yes

### Input

`*pb.StopEvent` from `dwell-connector`. Processes **both** `ARRIVAL` and `DEPARTURE` events.

### State

- **Store:** `InMemoryStateStore` with a 1-hour TTL.
- **Key:** `"agencyID-routeID-stopID-directionID-vehicleID"` — identifies a specific vehicle at a specific stop.
- **Value:** The `ARRIVAL` `*pb.StopEvent` for that vehicle/stop combination.

### Processing

Dwell time measures **how long a specific vehicle remained at a stop** (from arrival to departure).

On **ARRIVAL**:
1. Store the arrival event in state with a 1-hour TTL.
2. No output.

On **DEPARTURE**:
1. Look up the stored arrival for this vehicle/stop key.
2. If no arrival found, discard — cannot compute dwell without a matching arrival.
3. Compute: `dwell_seconds = departure_timestamp − arrival_timestamp`.
4. Delete the state entry (explicit cleanup; orphaned arrivals expire via TTL after 1 hour).
5. Emit a `DwellTimeEvent`.

### Output

`*pb.DwellTimeEvent` — zero or one event per DEPARTURE input.

```proto
message DwellTimeEvent {
  EventAttributes attributes = 1;
  int32 dwell_time_seconds = 4;
}
```

### Connectors

Terminal pipeline. Writes to `console-sink` and `sse-sink` only.

---

## 6. TravelTimeEventProcessor

**Source:** `processors/transit/travel_time_events/travel_time_event_processor.go`
**Pipeline type:** `travel_time`
**Stateful:** Yes

### Input

`*pb.StopEvent` from `travel-time-connector`. Only `ARRIVAL` events are processed; departures are discarded immediately.

### State

- **Store:** `InMemoryStateStore` with a 2-hour TTL (1-hour TTL per entry).
- **Key:** `"routeID-directionID-vehicleID-tripID"` — identifies a vehicle's journey on a specific trip.
- **Value:** The previous `ARRIVAL` `*pb.StopEvent` for that trip.

### Processing

Travel time measures **how long a vehicle took to travel between two consecutive stops** on the same trip, calculated as the difference between arrival timestamps.

1. Filter to ARRIVAL events only.
2. Build key from `route_id`, `direction_id`, `vehicle_id`, `trip_id`.
3. Look up the previous arrival for this trip.
4. If none found (first stop of the trip), store the current event and return — no output.
5. If the previous stop ID equals the current stop ID, skip — duplicate arrival, no travel occurred.
6. Compute: `travel_seconds = current_arrival_timestamp − previous_arrival_timestamp`.
7. Update state with the current arrival (overwriting the previous).
8. Emit a `TravelTimeEvent`.

### Output

`*pb.TravelTimeEvent` — zero or one event per ARRIVAL input.

```proto
message TravelTimeEvent {
  EventAttributes attributes = 1;  // includes origin_stop_id, destination_stop_id
  int32 travel_time_seconds = 4;
}
```

`attributes.origin_stop_id` is set to the previous stop; `attributes.destination_stop_id` is set to the current stop.

### Connectors

Terminal pipeline. Writes to `console-sink` and `sse-sink` only.

---

## State Store

All stateful processors use the `StateStore` interface defined in `statestore/state_store.go`, with the default implementation in `statestore/in_memory_store.go`.

```go
type StateStore interface {
    Get(key string, new func() proto.Message) (proto.Message, bool)
    Set(key string, msg proto.Message, ttl time.Duration) error
    Delete(key string)
    Close()
}
```

The in-memory implementation uses `sync.RWMutex` for thread safety and a background goroutine to periodically evict expired entries.

| Processor | State key pattern | TTL |
|---|---|---|
| `StopEventProcessor` | `vehicle_id` | 1 hour per entry |
| `HeadwayEventProcessor` | `routeID-directionID-stopID` | 1 hour per entry |
| `DwellEventProcessor` | `agencyID-routeID-stopID-directionID-vehicleID` | 1 hour (explicit delete on match) |
| `TravelTimeEventProcessor` | `routeID-directionID-vehicleID-tripID` | 1 hour per entry |

---

## Event Attributes

All event types embed `EventAttributes` (`api/v1/events/transit.proto`), which carries common contextual fields throughout the pipeline:

```proto
message EventAttributes {
  string agency_id = 1;
  string vehicle_id = 2;
  string route_id = 3;
  string stop_id = 4;
  string origin_stop_id = 5;
  string destination_stop_id = 6;
  string direction_id = 7;
  int32 stop_sequence = 11;
  StopStatus stop_status = 12;
  string trip_id = 13;
  string service_date = 15;
  google.protobuf.Timestamp timestamp = 16;
}
```

Each processor inherits and propagates these fields from upstream events, adding or replacing only the fields specific to its metric.
