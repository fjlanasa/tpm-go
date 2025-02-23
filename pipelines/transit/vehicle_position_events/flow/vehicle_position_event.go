package pipelines

import (
	"context"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VehiclePositionEventFlow struct {
	in  chan any
	out chan any
}

func NewVehiclePositionEventFlow(ctx context.Context) *VehiclePositionEventFlow {
	v := &VehiclePositionEventFlow{
		in:  make(chan any),
		out: make(chan any),
	}

	go v.doStream(ctx)

	return v
}

func (v *VehiclePositionEventFlow) In() chan<- any {
	return v.in
}

func (v *VehiclePositionEventFlow) Out() <-chan any {
	return v.out
}

func (v *VehiclePositionEventFlow) To(sink streams.Sink) {
	go v.transmit(sink)
}

func (v *VehiclePositionEventFlow) transmit(inlet streams.Inlet) {
	for element := range v.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (v *VehiclePositionEventFlow) doStream(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-v.in:
			if feedMessageEvent, ok := event.(*pb.FeedMessageEvent); ok {
				agencyId := feedMessageEvent.GetAgencyId()
				if feedMessage := feedMessageEvent.GetFeedMessage(); feedMessage != nil {
					for _, entity := range feedMessage.GetEntity() {
						var status pb.StopStatus
						switch entity.GetVehicle().GetCurrentStatus().Number() {
						case gtfs.VehiclePosition_IN_TRANSIT_TO.Number():
							status = pb.StopStatus_IN_TRANSIT_TO
						case gtfs.VehiclePosition_STOPPED_AT.Number():
							status = pb.StopStatus_STOPPED_AT
						case gtfs.VehiclePosition_INCOMING_AT.Number():
							status = pb.StopStatus_INCOMING_AT
						}
						vehicle := entity.GetVehicle()
						if vehicle != nil {
							v.out <- &pb.VehiclePositionEvent{
								AgencyId:      agencyId,
								EventId:       vehicle.GetVehicle().GetId(),
								VehicleId:     vehicle.GetVehicle().GetId(),
								VehicleLabel:  vehicle.GetVehicle().GetLabel(),
								RouteId:       vehicle.GetTrip().GetRouteId(),
								TripId:        vehicle.GetTrip().GetTripId(),
								DirectionId:   uint32(vehicle.GetTrip().GetDirectionId()),
								StopId:        vehicle.GetStopId(),
								StopStatus:    status,
								StopSequence:  int32(vehicle.GetCurrentStopSequence()),
								Latitude:      float64(vehicle.GetPosition().GetLatitude()),
								Longitude:     float64(vehicle.GetPosition().GetLongitude()),
								Timestamp:     timestamppb.New(time.Unix(int64(vehicle.GetTimestamp()), 0)),
								ServiceDate:   vehicle.GetTrip().GetStartDate(),
								BranchRouteId: vehicle.GetTrip().GetRouteId(),
								TrunkRouteId:  vehicle.GetTrip().GetRouteId(),
								ParentStation: vehicle.GetStopId(),
							}
						}
					}
				}
			}
		}
	}
}

func (v *VehiclePositionEventFlow) Via(flow streams.Flow) streams.Flow {
	go v.transmit(flow)
	return flow
}
