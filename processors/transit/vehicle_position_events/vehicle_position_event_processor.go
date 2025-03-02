package processors

import (
	"context"
	"strconv"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VehiclePositionEventProcessor struct {
	in  chan any
	out chan any
}

func NewVehiclePositionEventProcessor(ctx context.Context) *VehiclePositionEventProcessor {
	v := &VehiclePositionEventProcessor{
		in:  make(chan any),
		out: make(chan any),
	}

	go v.doStream(ctx)

	return v
}

func (v *VehiclePositionEventProcessor) In() chan<- any {
	return v.in
}

func (v *VehiclePositionEventProcessor) Out() <-chan any {
	return v.out
}

func (v *VehiclePositionEventProcessor) To(sink streams.Sink) {
	go v.transmit(sink)
}

func (v *VehiclePositionEventProcessor) transmit(inlet streams.Inlet) {
	for element := range v.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (v *VehiclePositionEventProcessor) doStream(ctx context.Context) {
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
							vehicleId := vehicle.GetVehicle().GetId()
							if vehicleId == "" {
								vehicleId = vehicle.GetTrip().GetTripId()
							}
							v.out <- &pb.VehiclePositionEvent{
								Attributes: &pb.EventAttributes{
									AgencyId:     agencyId,
									VehicleId:    vehicleId,
									RouteId:      vehicle.GetTrip().GetRouteId(),
									StopId:       vehicle.GetStopId(),
									TripId:       vehicle.GetTrip().GetTripId(),
									ServiceDate:  vehicle.GetTrip().GetStartDate(),
									DirectionId:  strconv.FormatUint(uint64(vehicle.GetTrip().GetDirectionId()), 10),
									StopSequence: int32(vehicle.GetCurrentStopSequence()),
									StopStatus:   status,
									Timestamp:    timestamppb.New(time.Unix(int64(vehicle.GetTimestamp()), 0)),
								},
								Latitude:  float64(vehicle.GetPosition().GetLatitude()),
								Longitude: float64(vehicle.GetPosition().GetLongitude()),
							}
						}
					}
				}
			}
		}
	}
}

func (v *VehiclePositionEventProcessor) Via(flow streams.Flow) streams.Flow {
	go v.transmit(flow)
	return flow
}
