# HeadwayEventProcessor

**File:** `processors/transit/headway_events/headway_event_processor.go`
**Stateful:** Yes

## Input

`*pb.StopEvent` — only `DEPARTURE` events are processed; arrivals are discarded immediately.

## State

- **Key:** `"routeID-directionID-stopID"` — identifies a directional stop on a route
- **Value:** The previous `DEPARTURE` `*pb.StopEvent` at that stop
- **TTL:** 1 hour per entry

## Processing

Headway measures the **time gap between consecutive vehicle departures** at the same directional stop on the same route.

1. Build the state key from `route_id`, `direction_id`, and `stop_id`.
2. Look up the previous departure for this key.
3. If none found (first departure seen), store the current event and return — no output.
4. If the previous departure is from the **same vehicle**, return — avoids false headways from repeated position updates.
5. Compute: `headway_seconds = current_timestamp − previous_timestamp`.
6. Update state with the current event.
7. Emit a `HeadwayTimeEvent`.

## Output

`*pb.HeadwayTimeEvent` — zero or one event per DEPARTURE input.

```proto
message HeadwayTimeEvent {
  EventAttributes attributes = 1;
  string leading_vehicle_id = 2;   // vehicle that departed previously
  string following_vehicle_id = 3; // current vehicle
  int32 headway_seconds = 4;
}
```
