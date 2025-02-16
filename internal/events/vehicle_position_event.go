package events

import pb "github.com/fjlanasa/tpm-go/api/v1/events"

type VehiclePositionEvent struct {
	Event
	pb.VehiclePositionEvent
}

func (vpe *VehiclePositionEvent) GetAttributes() map[string]any {
	return map[string]any{
		"latitude":  vpe.GetLatitude(),
		"longitude": vpe.GetLongitude(),
	}
}
