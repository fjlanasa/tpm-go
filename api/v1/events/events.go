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
		"agency_id":     e.GetAgencyId(),
		"route_id":      e.GetRouteId(),
		"trip_id":       e.GetTripId(),
		"direction_id":  e.GetDirectionId(),
		"stop_id":       e.GetStopId(),
		"stop_sequence": e.GetStopSequence(),
		"latitude":      e.GetLatitude(),
		"longitude":     e.GetLongitude(),
		"timestamp":     e.GetTimestamp().AsTime().Unix(),
	}
}

func (e *StopEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":     e.GetAgencyId(),
		"route_id":      e.GetRouteId(),
		"trip_id":       e.GetTripId(),
		"direction_id":  e.GetDirectionId(),
		"stop_id":       e.GetStopId(),
		"stop_sequence": e.GetStopSequence(),
		"event_type":    e.GetEventType().String(),
		"timestamp":     e.GetTimestamp().AsTime().Unix(),
	}
}

func (e *DwellTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":          e.GetAgencyId(),
		"route_id":           e.GetRouteId(),
		"trip_id":            e.GetTripId(),
		"direction_id":       e.GetDirectionId(),
		"stop_id":            e.GetStopId(),
		"stop_sequence":      e.GetStopSequence(),
		"dwell_time_seconds": e.GetDwellTimeSeconds(),
		"timestamp":          e.GetTimestamp().AsTime().Unix(),
	}
}

func (e *HeadwayTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":            e.GetAgencyId(),
		"route_id":             e.GetRouteId(),
		"direction_id":         e.GetDirectionId(),
		"stop_id":              e.GetStopId(),
		"headway_time_seconds": e.GetHeadwayBranchSeconds(),
		"timestamp":            e.GetTimestamp().AsTime().Unix(),
	}
}

func (e *TravelTimeEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id":           e.GetAgencyId(),
		"route_id":            e.GetRouteId(),
		"trip_id":             e.GetTripId(),
		"direction_id":        e.GetDirectionId(),
		"origin_stop_id":      e.GetFromStopId(),
		"destination_stop_id": e.GetToStopId(),
		"travel_time_seconds": e.GetTravelTimeSeconds(),
		"timestamp":           e.GetTimestamp().AsTime().Unix(),
	}
}
