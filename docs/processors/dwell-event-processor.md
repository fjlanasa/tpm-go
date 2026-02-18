# DwellEventProcessor

**File:** `processors/transit/dwell_events/dwell_event_processor.go`
**Stateful:** Yes

## Input

`*pb.StopEvent` — processes both `ARRIVAL` and `DEPARTURE` events.

## State

- **Key:** `"agencyID-routeID-stopID-directionID-vehicleID"` — identifies a specific vehicle at a specific stop
- **Value:** The `ARRIVAL` `*pb.StopEvent` for that vehicle/stop
- **TTL:** 1 hour (orphaned arrivals without a matching departure expire automatically)

## Processing

Dwell time measures **how long a specific vehicle remained at a stop**, from arrival to departure.

On **ARRIVAL**:
1. Store the arrival event in state.
2. No output.

On **DEPARTURE**:
1. Look up the stored arrival for this vehicle/stop key.
2. If no arrival found, discard — cannot compute dwell without a paired arrival.
3. Compute: `dwell_seconds = departure_timestamp − arrival_timestamp`.
4. Delete the state entry.
5. Emit a `DwellTimeEvent`.

## Output

`*pb.DwellTimeEvent` — zero or one event per DEPARTURE input.

```proto
message DwellTimeEvent {
  EventAttributes attributes = 1;
  int32 dwell_time_seconds = 4;
}
```
