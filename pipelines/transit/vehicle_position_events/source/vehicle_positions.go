package vehicle_position_events

import (
	"context"
	"log"
	"net/http"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/sources"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type VehiclePositionsSource struct {
	config     config.SourceConfig
	feedSource sources.Source
	out        chan any
	ctx        context.Context
}

func NewVehiclePositionsSource(ctx context.Context, cfg config.SourceConfig) *VehiclePositionsSource {
	feedSource, err := sources.NewSource[*gtfs.FeedMessage](ctx, cfg, func() *gtfs.FeedMessage {
		return &gtfs.FeedMessage{}
	})
	if err != nil {
		log.Fatal(err)
	}
	source := &VehiclePositionsSource{ctx: ctx, config: cfg, feedSource: feedSource, out: make(chan any)}
	go source.init()
	return source
}

func (v *VehiclePositionsSource) init() {
	out := v.feedSource.Out()
	for {
		select {
		case <-v.ctx.Done():
			return
		case feed := <-out:
			feedMessage := feed.(*gtfs.FeedMessage)
			for _, entity := range feedMessage.Entity {
				var status pb.StopStatus
				switch entity.GetVehicle().GetCurrentStatus().Number() {
				case gtfs.VehiclePosition_IN_TRANSIT_TO.Number():
					status = pb.StopStatus_IN_TRANSIT_TO
				case gtfs.VehiclePosition_STOPPED_AT.Number():
					status = pb.StopStatus_STOPPED_AT
				case gtfs.VehiclePosition_INCOMING_AT.Number():
					status = pb.StopStatus_INCOMING_AT
				}
				if entity.Vehicle != nil {
					v.out <- &pb.VehiclePositionEvent{
						AgencyId:     v.config.AgencyID,
						EventId:      entity.GetVehicle().GetVehicle().GetId(),
						VehicleId:    entity.GetVehicle().GetVehicle().GetId(),
						VehicleLabel: entity.GetVehicle().GetVehicle().GetLabel(),
						RouteId:      entity.GetVehicle().GetTrip().GetRouteId(),
						TripId:       entity.GetVehicle().GetTrip().GetTripId(),
						DirectionId:  uint32(entity.GetVehicle().GetTrip().GetDirectionId()),
						StopId:       entity.GetVehicle().GetStopId(),
						StopStatus:   status,
						StopSequence: int32(entity.GetVehicle().GetCurrentStopSequence()),
						Latitude:     float64(entity.GetVehicle().GetPosition().GetLatitude()),
						Longitude:    float64(entity.GetVehicle().GetPosition().GetLongitude()),
					}
				}
			}
		}
	}
}

func (v *VehiclePositionsSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(v, operator)
	return operator
}

func (v *VehiclePositionsSource) Out() <-chan any {
	return v.out
}
