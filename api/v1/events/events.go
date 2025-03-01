package events

import (
	"fmt"
	"strings"
)

// Base interface for all events
type Event interface {
	GetAttributes() *EventAttributes
	GetValues() map[string]any
}

func GetEventAttributesMap(e Event) map[string]any {
	return map[string]any{
		"event_type":            strings.TrimPrefix(fmt.Sprintf("%T", e), "*events."),
		"agency_id":             e.GetAttributes().GetAgencyId(),
		"vehicle_id":            e.GetAttributes().GetVehicleId(),
		"route_id":              e.GetAttributes().GetRouteId(),
		"stop_id":               e.GetAttributes().GetStopId(),
		"origin_stop_id":        e.GetAttributes().GetOriginStopId(),
		"destination_stop_id":   e.GetAttributes().GetDestinationStopId(),
		"direction_id":          e.GetAttributes().GetDirectionId(),
		"direction":             e.GetAttributes().GetDirection(),
		"direction_destination": e.GetAttributes().GetDirectionDestination(),
		"parent_station":        e.GetAttributes().GetParentStation(),
		"stop_sequence":         e.GetAttributes().GetStopSequence(),
		"stop_status":           e.GetAttributes().GetStopStatus(),
		"trip_id":               e.GetAttributes().GetTripId(),
		"service_date":          e.GetAttributes().GetServiceDate(),
		"timestamp":             e.GetAttributes().GetTimestamp().AsTime(),
	}
}

func GetEventValuesMap(e Event) map[string]any {
	return e.GetValues()
}

func GetEventMap(e Event) map[string]any {
	attributes := GetEventAttributesMap(e)
	values := GetEventValuesMap(e)
	combined := make(map[string]any)
	for k, v := range attributes {
		combined[k] = v
	}
	for k, v := range values {
		combined[k] = v
	}
	return combined
}

func (e *FeedMessageEvent) GetAttributes() map[string]any {
	return map[string]any{
		"agency_id": e.AgencyId,
		"timestamp": e.FeedMessage.Header.GetTimestamp(),
	}
}

func (e *VehiclePositionEvent) GetValues() map[string]any {
	return map[string]any{
		"latitude":  e.GetLatitude(),
		"longitude": e.GetLongitude(),
	}
}

func (e *StopEvent) GetValues() map[string]any {
	return map[string]any{
		"stop_event_type": e.GetStopEventType().String(),
	}
}

func (e *DwellTimeEvent) GetValues() map[string]any {
	return map[string]any{
		"arrival_time":       e.GetArrivalTime().AsTime().Unix(),
		"departure_time":     e.GetDepartureTime().AsTime().Unix(),
		"dwell_time_seconds": e.GetDwellTimeSeconds(),
	}
}

func (e *HeadwayTimeEvent) GetValues() map[string]any {
	return map[string]any{
		"leading_vehicle_id":   e.GetLeadingVehicleId(),
		"following_vehicle_id": e.GetFollowingVehicleId(),
		"headway_time_seconds": e.GetHeadwaySeconds(),
	}
}

func (e *TravelTimeEvent) GetValues() map[string]any {
	return map[string]any{
		"start_time":          e.GetStartTime().AsTime().Unix(),
		"end_time":            e.GetEndTime().AsTime().Unix(),
		"travel_time_seconds": e.GetTravelTimeSeconds(),
	}
}
