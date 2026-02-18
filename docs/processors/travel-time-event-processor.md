# TravelTimeEventProcessor

**File:** `processors/transit/travel_time_events/travel_time_event_processor.go`
**Stateful:** Yes

## Input

`*pb.StopEvent` — only `ARRIVAL` events are processed; departures are discarded immediately.

## State

- **Key:** `"routeID-directionID-vehicleID-tripID"` — identifies a vehicle's journey on a specific trip
- **Value:** The previous `ARRIVAL` `*pb.StopEvent` for that trip
- **TTL:** 1 hour per entry

## Processing

Travel time measures **how long a vehicle took to travel between two consecutive stops** on the same trip, using the difference between arrival timestamps.

1. Build the state key from `route_id`, `direction_id`, `vehicle_id`, and `trip_id`.
2. Look up the previous arrival for this trip.
3. If none found (first stop of the trip), store the current event and return — no output.
4. If the previous stop ID equals the current stop ID, return — duplicate arrival, no travel occurred.
5. Compute: `travel_seconds = current_arrival_timestamp − previous_arrival_timestamp`.
6. Update state with the current arrival.
7. Emit a `TravelTimeEvent`.

## Output

`*pb.TravelTimeEvent` — zero or one event per ARRIVAL input.

```proto
message TravelTimeEvent {
  EventAttributes attributes = 1; // origin_stop_id and destination_stop_id are set
  int32 travel_time_seconds = 4;
}
```

`attributes.origin_stop_id` is the previous stop; `attributes.destination_stop_id` is the current stop.
