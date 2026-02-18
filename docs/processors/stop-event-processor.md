# StopEventProcessor

**File:** `processors/transit/stop_events/stop_event.go`
**Stateful:** Yes

## Input

`*pb.VehiclePositionEvent`

## State

- **Key:** `vehicle_id`
- **Value:** The previous `*pb.VehiclePositionEvent` for that vehicle
- **TTL:** 1 hour per entry

## Processing

Implements a **state machine** over each vehicle's `StopStatus` transitions to detect when a vehicle arrives at or departs from a stop.

On each event:

1. Retrieve the previous state for the vehicle.
2. Store the current event as the new state.
3. If no previous state exists (first event for this vehicle), return — no output.
4. If both `stop_status` and `stop_id` are unchanged, return — no output.
5. Apply transition logic:

| Previous status | Current status | Stop changed | Output |
|---|---|---|---|
| `IN_TRANSIT_TO` or `INCOMING_AT` | `STOPPED_AT` | — | ARRIVAL at current stop |
| `STOPPED_AT` | `IN_TRANSIT_TO` | — | DEPARTURE at previous stop |
| `IN_TRANSIT_TO` | `IN_TRANSIT_TO` | Yes | ARRIVAL at previous stop + DEPARTURE at previous stop |

The third row handles vehicles that skip `STOPPED_AT` entirely — an inferred arrival and departure are both emitted. The arrival uses the previous event's attributes; the departure uses the current event's attributes with the previous stop ID.

Events with an empty `vehicle_id` or `stop_id` are discarded.

## Output

`*pb.StopEvent` — zero, one, or two events per input.

```proto
message StopEvent {
  EventAttributes attributes = 1;
  enum EventType { UNKNOWN = 0; ARRIVAL = 1; DEPARTURE = 2; }
  EventType stop_event_type = 14;
}
```
