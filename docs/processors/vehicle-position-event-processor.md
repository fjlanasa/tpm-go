# VehiclePositionEventProcessor

**File:** `processors/transit/vehicle_position_events/vehicle_position_event_processor.go`
**Stateful:** No

## Input

`*pb.FeedMessageEvent`

## Processing

Iterates over every entity in the `FeedMessage`. For each entity with a vehicle:

1. Maps the GTFS `CurrentStatus` enum to the internal `StopStatus` enum:
   - `IN_TRANSIT_TO` → `pb.StopStatus_IN_TRANSIT_TO`
   - `STOPPED_AT` → `pb.StopStatus_STOPPED_AT`
   - `INCOMING_AT` → `pb.StopStatus_INCOMING_AT`
2. Resolves vehicle ID: uses `vehicle.id` if present, falls back to `trip.trip_id`.
3. Converts the GTFS Unix timestamp to a `google.protobuf.Timestamp`.
4. Emits one `VehiclePositionEvent` per vehicle entity.

## Output

`*pb.VehiclePositionEvent` — one per vehicle entity in the feed.

```proto
message VehiclePositionEvent {
  EventAttributes attributes = 1;
  double latitude = 2;
  double longitude = 3;
}
```

`EventAttributes` fields populated: `agency_id`, `vehicle_id`, `route_id`, `stop_id`, `trip_id`, `service_date`, `direction_id`, `stop_sequence`, `stop_status`, `timestamp`.
