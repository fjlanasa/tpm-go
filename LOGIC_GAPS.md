# Processing Flow Logic Gaps Analysis

This document identifies logic gaps, edge cases, and potential issues in the TPM-Go processing flow components.

---

## 1. StopEventProcessor (`processors/transit/stop_events/stop_event.go`)

### 1a. INCOMING_AT → STOPPED_AT at a *different* stop is treated as arrival — but may be wrong stop

**Lines 169-173**: An ARRIVAL is emitted when transitioning from `IN_TRANSIT_TO`/`INCOMING_AT` to `STOPPED_AT`. But if the `stop_id` changed between the previous and current event (e.g., the feed skipped an update and the vehicle is now at a completely different stop), the code still emits an arrival at the *current* stop without acknowledging that the vehicle may have silently departed the previous stop.

**Impact**: Missed DEPARTURE events for stops the vehicle passed through between feed polls.

### 1b. No handling of INCOMING_AT → IN_TRANSIT_TO transition

**Lines 169-182**: The switch only handles `STOPPED_AT` and `IN_TRANSIT_TO` as the *current* status. If a vehicle goes from `INCOMING_AT` to `IN_TRANSIT_TO` (approaching a stop then skipping it), no event is emitted. This is arguably correct (no actual stop was made), but the synthesized arrival/departure logic at lines 177-181 only fires for `IN_TRANSIT_TO → IN_TRANSIT_TO` with a stop change, not `INCOMING_AT → IN_TRANSIT_TO` with a stop change.

**Impact**: If a vehicle was `INCOMING_AT stop A` then suddenly becomes `IN_TRANSIT_TO stop B`, no events are emitted — a gap in the detection logic.

### 1c. Synthesized arrival uses wrong event data for timestamp

**Line 179**: When synthesizing an arrival for the `IN_TRANSIT_TO → IN_TRANSIT_TO` (stop changed) case, the code passes `previous` (the old VehiclePositionEvent) to `makeStopEvent`. The timestamp on this event is the *transit* timestamp, not the actual arrival time. The real arrival at that stop happened somewhere between the previous and current feed updates. The synthesized departure on line 180 uses `event` (current data), which also uses the transit timestamp of the *new* destination — not the departure time from the old stop.

**Impact**: Both the synthesized arrival and departure timestamps are approximations that don't reflect actual stop/departure times. This cascades into incorrect dwell times and travel times downstream.

### 1d. No handling of STOPPED_AT → STOPPED_AT with different stop_id

**Lines 164-167**: If `prevAttrs.GetStopStatus() == attrs.GetStopStatus()` AND `prevAttrs.GetStopId() == attrs.GetStopId()`, the processor returns early. But if the status is the same (`STOPPED_AT`) and the stop_id *differs*, the code falls through to the switch at line 169. The `STOPPED_AT` case (line 170-173) only emits an arrival if the previous status was `IN_TRANSIT_TO` or `INCOMING_AT` — but here the previous status is `STOPPED_AT`. No departure from the previous stop or arrival at the new stop is emitted.

**Impact**: If the feed reports a vehicle jumping from `STOPPED_AT stop A` to `STOPPED_AT stop B` (skipped transit phase), both a departure from A and arrival at B are silently dropped.

### 1e. State is stored *before* checking whether a transition happened

**Lines 139-145**: The current event is stored in the state store before the transition logic runs. If the state store `Set` fails (returns error), the function returns early (line 144), and no events are emitted. But if it succeeds, the old state is already overwritten. This means if the `process` function panics after line 145, the previous state is lost.

This also means the state store always contains the *latest* position, even if no event was emitted — which is the desired behavior, but the error-return-early on line 144 means a transient `Set` failure causes the entire update (including the transition check) to be dropped.

---

## 2. HeadwayEventProcessor (`processors/transit/headway_events/headway_event_processor.go`)

### 2a. Unsafe type assertion without nil/ok check

**Line 91**: `stopEvent := state.(*pb.StopEvent)` — this is a bare type assertion (no `, ok` guard). If the state store returns a message of a different type (e.g., due to key collision across processor types sharing a store, or store corruption), this will panic.

**Impact**: Runtime panic crashes the headway processing goroutine, which closes the output channel and cascades downstream.

### 2b. No validation for negative or zero headway

**Line 106**: `headwaySeconds := int32(currentTime.Sub(lastTime).Seconds())` — if the current event's timestamp is *before* the stored event's timestamp (due to out-of-order feed data, clock skew, or feed replay), the headway will be negative. No validation prevents this negative value from being emitted.

**Impact**: Downstream consumers receive nonsensical negative headway values.

### 2c. No context cancellation handling in doStream

**Lines 129-137**: The `doStream` method only ranges over `f.in` — there is no `select` on `ctx.Done()`. Compare with `StopEventProcessor.doStream` (lines 188-202) which properly listens for context cancellation. If the context is cancelled but the input channel isn't closed, the headway processor's goroutine will block indefinitely.

**Impact**: Goroutine leak on shutdown if the upstream connector doesn't close its channel promptly.

### 2d. State store error ignored on first-vehicle Set

**Line 94**: `_ = f.headwayStates.Set(key.String(), event, time.Hour)` — the error is silently discarded. If this Set fails, the next vehicle at this stop/route/direction will also be treated as "first arrival" since no state exists, and headway calculation will be permanently skipped for this key until a Set succeeds.

Similarly at **line 124**: `_ = f.headwayStates.Set(key.String(), event, time.Hour)` — if this fails after emitting the headway event, the *next* calculation will use stale data.

---

## 3. DwellEventProcessor (`processors/transit/dwell_events/dwell_event_processor.go`)

### 3a. No validation for negative dwell time

**Line 102**: `dwellSeconds := int32(event.GetAttributes().GetTimestamp().AsTime().Sub(currentState.(*pb.StopEvent).GetAttributes().GetTimestamp().AsTime()).Seconds())` — if the DEPARTURE timestamp is before the ARRIVAL timestamp (out-of-order events, clock issues), the dwell time will be negative.

**Impact**: Negative dwell times emitted to downstream consumers.

### 3b. Unsafe type assertion in dwell calculation

**Line 102**: `currentState.(*pb.StopEvent)` — bare assertion without `, ok` check. If `currentState` is not a `*pb.StopEvent`, this panics.

### 3c. DEPARTURE without prior ARRIVAL silently dropped (correct but undocumented)

**Lines 100-118**: If a `DEPARTURE` event arrives and `found` is false (no stored ARRIVAL), nothing happens. This is correct behavior (can't compute dwell without arrival), but there's no logging or metric to track how often this occurs, making it hard to diagnose data quality issues.

### 3d. Arrival overwrites previous arrival without emitting anything

**Line 117**: If two consecutive ARRIVAL events come for the same key (e.g., the stop event processor emits duplicate arrivals), the second ARRIVAL silently overwrites the first in the state store. The dwell time will then be computed from the *second* arrival to the departure, which may be shorter than the actual dwell.

### 3e. No context cancellation handling in doStream

**Lines 121-129**: Same issue as HeadwayEventProcessor — `doStream` only ranges over `d.in` with no `select` on context done.

### 3f. AgencyId missing from emitted DwellTimeEvent

**Lines 104-113**: The `DwellTimeEvent` attributes include `RouteId`, `StopId`, `DirectionId`, `VehicleId`, and `Timestamp`, but `AgencyId` is omitted. The `StopEvent` input has `AgencyId` available via `event.GetAttributes().GetAgencyId()`, but it's not propagated.

**Impact**: DwellTimeEvents cannot be filtered or attributed to a specific agency downstream, and the SSE subscription filtering by `agency_id` won't work for dwell events.

---

## 4. TravelTimeEventProcessor (`processors/transit/travel_time_events/travel_time_event_processor.go`)

### 4a. No validation for negative travel time

**Line 98**: Same pattern as headway/dwell — `travelSeconds` can be negative if timestamps are out of order.

### 4b. No context cancellation handling in doStream

**Lines 119-130**: Same goroutine leak risk as Headway and Dwell processors.

### 4c. State always updated even if no event was emitted

**Line 116**: `_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)` is called unconditionally after the found-check block. If two consecutive arrivals are at the *same* stop (line 97 check fails), the state is still updated. This means the third arrival (at a different stop) will compute travel time from the second arrival (same stop) to itself — which would capture the time spent stopped rather than traveling.

**Impact**: If arrivals at the same stop repeat (e.g., loop routes or duplicate events), travel time between stops becomes inflated.

### 4d. ServiceDate and StopSequence not propagated

**Lines 100-112**: The emitted `TravelTimeEvent` attributes include `AgencyId`, `RouteId`, `TripId`, `DirectionId`, `VehicleId`, `OriginStopId`, `DestinationStopId`, and `Timestamp`, but `ServiceDate` and `StopSequence` are omitted. `ServiceDate` in particular is important for analyzing travel times by service day.

---

## 5. VehiclePositionEventProcessor (`processors/transit/vehicle_position_events/vehicle_position_event_processor.go`)

### 5a. No handling of UNKNOWN vehicle stop status

**Lines 63-71**: The switch maps `IN_TRANSIT_TO`, `STOPPED_AT`, and `INCOMING_AT` but has no `default` case. If the GTFS feed reports an unrecognized status value, `status` defaults to `pb.StopStatus_UNKNOWN` (proto zero value). The downstream `StopEventProcessor` doesn't handle `UNKNOWN` status in its transition logic, so vehicles with unknown status will never generate stop events.

**Impact**: Silent data loss for vehicles with non-standard or missing status.

### 5b. Non-vehicle entities silently processed for status

**Lines 63-71**: The status is determined from `entity.GetVehicle().GetCurrentStatus()` *before* the `vehicle != nil` check on line 73. If the entity doesn't have a vehicle position (e.g., it's a trip update or alert entity), `entity.GetVehicle()` returns a zero value, and the status switch runs against its default number. This doesn't cause a crash (protobuf getters are nil-safe), but it's unnecessary work.

### 5c. Entities without a stop_id produce events with empty stop_id

**Lines 78-93**: If `vehicle.GetStopId()` returns an empty string (some feeds don't always provide stop_id), a `VehiclePositionEvent` is still emitted with an empty `StopId`. This flows into `StopEventProcessor`, which returns early on empty vehicle_id (line 123-126) but does NOT check for empty stop_id. The state store will contain entries with empty stop_id, and transition logic will run with empty stop IDs.

**Impact**: Meaningless stop events (arrivals/departures at "") propagated to headway, dwell, and travel time processors. The dwell state key, headway key, and travel time key would all contain empty stop components, potentially causing key collisions.

### 5d. DirectionId is always "0" when the field is not set

**Line 86**: `strconv.FormatUint(uint64(vehicle.GetTrip().GetDirectionId()), 10)` — if `DirectionId` is not set in the GTFS feed, the proto default is `0`, so this always produces the string `"0"`. This is technically correct but makes it impossible to distinguish between "direction 0" and "direction not specified". Downstream processors use direction_id in composite keys, so unset directions will collide with actual direction 0.

---

## 6. FeedMessageProcessor (`processors/transit/feed_message_events/feed_message_event_processor.go`)

### 6a. No HTTP status code validation upstream

The FeedMessageProcessor receives raw bytes from the HTTP source. The `HTTPSource` (sources/http_source.go:50-61) reads the response body regardless of HTTP status code — a 404, 500, or 301 response body would be passed to the FeedMessageProcessor as bytes. The processor would then fail to unmarshal it as a protobuf and silently `continue`.

**Impact**: Transient server errors cause silent data gaps with no logging.

### 6b. Output channel closed inconsistently

**Lines 26-57**: If `ctx.Done()` fires, the function returns *without* closing `f.out`. But if `f.in` is closed (line 31-34), `f.out` IS closed. Compare with `VehiclePositionEventProcessor.doStream` which uses `defer close(v.out)`. This inconsistency means context cancellation leaves `f.out` open, potentially causing goroutine leaks in downstream processors that range over it.

---

## 7. InMemoryStateStore (`statestore/in_memory_store.go`)

### 7a. Read-under-write race in Get

**Lines 37-50**: `Get` acquires a read lock, but if the entry is expired (line 42), it returns `new()` WITHOUT deleting the expired entry. The expired entry remains in the map until the next expiry sweep. This isn't a correctness bug, but it means:
- The expired entry continues consuming memory until the next tick
- Two concurrent readers could both see the entry as expired and both call `new()`, which is fine since it's read-only

### 7b. Expiry sweep interval equals TTL

**Line 78**: `ticker := time.NewTicker(s.ttl)` — the sweep runs once per TTL period. If the TTL is 2 hours, expired entries can linger for up to 2 additional hours before cleanup. For a more responsive cleanup, the sweep interval should be a fraction of the TTL (e.g., TTL/10).

**Impact**: Memory grows unboundedly between sweeps in high-cardinality scenarios.

### 7c. Per-entry TTL overridden by per-key TTL on Set

**Lines 52-64**: Each `Set` call can specify its own TTL, and the `InMemoryState` struct stores it. But the entry's `ttl` field is never actually read again — the `expiration` field is what matters. The `ttl` field in `InMemoryState` is dead code.

---

## 8. RedisStateStore (`statestore/redis_store.go`)

### 8a. Stored context used for all operations

**Lines 34-44**: The `RedisStateStore` captures the context at construction time and uses it for ALL subsequent Get/Set/Delete operations. If this context is cancelled (e.g., on shutdown), all state store operations will fail with context errors even if the caller's local context is still active. This is inconsistent with how the processors use the store — they don't pass their own context to state store methods.

### 8b. No connection health check or retry logic

Redis operations can fail transiently (network blips, connection timeouts). There's no retry logic — a single failed Get/Set silently drops data.

---

## 9. Pipeline / Graph Orchestration

### 9a. Connector channel shared across multiple sinks via FanOut

**`pipelines/pipeline.go:69-74`**: The pipeline merges sources, routes through the processor, then uses `flow.FanOut` to distribute to multiple sinks. Each sink gets its own copy of the stream. But looking at the default config, the `se-pipeline` fans out to `headway-connector`, `dwell-connector`, `travel-time-connector`, `console-sink`, and `sse-sink`. The `FanOut` creates independent flows for each sink, which means each `StopEvent` is sent to all five sinks independently. This is correct behavior.

However, connector channels are **unbuffered** (`make(chan any)` in `graphs/graph.go:28`). If any downstream pipeline (e.g., headway) is slow, its connector channel blocks, which blocks the FanOut goroutine for that sink. Since FanOut goroutines are independent, this doesn't block other sinks — but it does mean a slow consumer can cause its connector to back up, potentially causing the FanOut goroutine to block indefinitely.

**Impact**: A stalled downstream pipeline causes unbounded goroutine accumulation in the FanOut stage.

### 9b. No state store definition in config yields nil state store silently

**`graphs/graph.go:70-82`**: When iterating pipeline configs, `stateStoresByID[cfg.StateStore]` is used to look up the state store. If `cfg.StateStore` is empty or references a non-existent ID, Go's map lookup returns `nil`. This `nil` is passed to `processors.NewProcessor`, which passes it to the specific processor constructor. The processors have fallback logic to create a default in-memory store when `stateStore` is nil — so this works, but the config references state stores that are never defined in the `state_stores` section of the YAML (the default.yaml has no `state_stores` section at all). This means *every* processor creates its own ad-hoc in-memory store, and these stores are NOT tracked by `graph.stateStores`, so `graph.Close()` never calls `Close()` on them.

**Impact**: The background expiry goroutines in the ad-hoc in-memory stores are never stopped on shutdown, causing goroutine leaks.

### 9c. NewSink returns nil for unknown sink types

**`sinks/sink.go:14-28`**: `NewSink` returns `nil` for unrecognized sink types. `NewPipeline` doesn't check for nil sinks — it would add `nil` to the sinks slice. When `pipeline.Run()` calls `sinkFlows[i].To(sink)` with a nil sink, this would panic.

### 9d. Pipeline ordering is non-deterministic

**`graphs/graph.go:70`**: Iterating `cfg.Pipelines` (a `map[ID]PipelineConfig`) is non-deterministic in Go. Pipelines that produce data (e.g., feed_message) must start before pipelines that consume it (e.g., vehicle_position). Since all pipelines are launched as goroutines at the same time in `graph.Run()`, this isn't strictly a problem — goroutines can start in any order. But if a producing pipeline starts and emits data before the consuming pipeline's goroutine has set up its channel reads, events could be lost or the send could block on unbuffered channels.

In practice, this is mitigated because the connector channels are shared references and the consuming pipeline's source goroutine will eventually start reading. But there is a race window.

---

## 10. Event Server (`event_server/event_server.go`)

### 10a. Broadcast blocks on slow subscribers

**Lines 100-120**: `Broadcast` holds a read lock and sends to each client's channel synchronously (`client.Channel <- eventMap` on line 117). Subscriber channels are unbuffered (`make(chan any)` on line 40). If a subscriber is slow to read, the broadcast blocks, holding the read lock and preventing ALL other broadcasts and subscribe/unsubscribe operations.

**Impact**: A single slow SSE client stalls the entire event pipeline.

### 10b. Debug print statements in production code

**Lines 115-116**: `fmt.Println("match")` and `fmt.Println(eventMap)` are debug statements that should not be in production code. They output to stdout for every matching event broadcast.

### 10c. Subscription filter comparison uses string equality on non-string values

**Lines 108-110**: `client.Subscription` maps are `map[string]string`, but `eventMap` (from `GetEventMap`) contains `map[string]any` with mixed types (strings, int32, time.Time, StopStatus enum). The comparison `v != eventMap[k]` compares a string to an `any` — this will ALWAYS be `match = false` for non-string fields because `string != int32` in Go's `!=` operator.

**Impact**: Subscription filters on fields like `stop_sequence`, `timestamp`, or `stop_status` will never match, effectively making those filters non-functional.

---

## 11. Cross-Cutting Issues

### 11a. Inconsistent channel closing across processors

- `FeedMessageProcessor`: closes `out` when `in` is closed, but NOT on context cancellation
- `VehiclePositionEventProcessor`: uses `defer close(v.out)` — always closes
- `StopEventProcessor`: uses `defer close(s.out)` — always closes
- `HeadwayEventProcessor`: uses `defer close(f.out)` — always closes, but no ctx select
- `DwellEventProcessor`: uses `defer close(d.out)` — always closes, but no ctx select
- `TravelTimeEventProcessor`: uses `defer close(f.out)` — always closes, but no ctx select

The `FeedMessageProcessor` is the odd one out. On context cancellation, its `out` channel stays open, causing downstream goroutines ranging over it to hang.

### 11b. No backpressure mechanism

All channels are unbuffered. If a downstream processor is slower than upstream, sends block. Since each processor runs in its own goroutine, this creates implicit backpressure. However, the HTTP source keeps polling on its ticker regardless of whether the previous event was consumed. If the `FeedMessageProcessor` is blocked on sending to its output (because the vehicle position processor is backed up), the HTTP source's `s.out <- body` will block, and the ticker will drift. This isn't a bug per se, but it means the polling interval becomes "at least N seconds" rather than "every N seconds."

### 11c. Proto event fields defined but never populated

The protobuf schema defines fields in `DwellTimeEvent` (`arrival_time`, `departure_time`) and `TravelTimeEvent` (`start_time`, `end_time`) that are never populated by their respective processors. The processors only set the `*_seconds` fields.

**Impact**: Consumers relying on the `arrival_time`/`departure_time` or `start_time`/`end_time` fields will get zero-value timestamps.
