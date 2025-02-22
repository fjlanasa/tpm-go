package events

// Base interface for all events
type Event interface {
	GetAgencyId() string
	GetAttributes() map[string]any
}

func (e *FeedMessageEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id": e.AgencyId,
		"timestamp": e.FeedMessage.Header.GetTimestamp(),
	}
}

func (e *VehiclePositionEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":     e.AgencyId,
		"route_id":      e.RouteId,
		"trip_id":       e.TripId,
		"direction_id":  e.DirectionId,
		"stop_id":       e.StopId,
		"stop_sequence": e.StopSequence,
		"latitude":      e.Latitude,
		"longitude":     e.Longitude,
	}
}

func (e *StopEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":     e.AgencyId,
		"route_id":      e.RouteId,
		"trip_id":       e.TripId,
		"direction_id":  e.DirectionId,
		"stop_id":       e.StopId,
		"stop_sequence": e.StopSequence,
		"event_type":    e.EventType.String(),
		"timestamp":     e.Timestamp,
	}
}

func (e *DwellTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":          e.AgencyId,
		"route_id":           e.RouteId,
		"trip_id":            e.TripId,
		"direction_id":       e.DirectionId,
		"stop_id":            e.StopId,
		"stop_sequence":      e.StopSequence,
		"dwell_time_seconds": e.DwellTimeSeconds,
		"timestamp":          e.Timestamp,
	}
}

func (e *HeadwayTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":            e.AgencyId,
		"route_id":             e.RouteId,
		"direction_id":         e.DirectionId,
		"stop_id":              e.StopId,
		"headway_time_seconds": e.HeadwayBranchSeconds,
		"timestamp":            e.Timestamp,
	}
}

func (e *TravelTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":           e.AgencyId,
		"route_id":            e.RouteId,
		"trip_id":             e.TripId,
		"direction_id":        e.DirectionId,
		"origin_stop_id":      e.FromStopId,
		"destination_stop_id": e.ToStopId,
		"travel_time_seconds": e.TravelTimeSeconds,
		"timestamp":           e.Timestamp,
	}
}
