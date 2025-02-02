package vehicle_position_events

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"google.golang.org/protobuf/proto"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type VehiclePositionsSource struct {
	agency_id string
	feed_url  string
	interval  time.Duration
	out       chan any
	client    HTTPClient
}

func NewVehiclePositionsSource(agency_id string, feed_url string, interval time.Duration) *VehiclePositionsSource {
	source := &VehiclePositionsSource{
		agency_id: agency_id,
		feed_url:  feed_url,
		interval:  interval,
		out:       make(chan any),
		client:    http.DefaultClient,
	}
	go source.init()
	return source
}

func (v *VehiclePositionsSource) init() {
	fmt.Println("Fetching every", v.interval)
	ticker := time.NewTicker(v.interval)

	for range ticker.C {
		resp, err := v.client.Get(v.feed_url)
		if err != nil {
			log.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		feed := gtfs.FeedMessage{}
		err = proto.Unmarshal(body, &feed)
		if err != nil {
			log.Fatal(err)
		}

		for _, entity := range feed.Entity {
			var status pb.StopStatus
			switch entity.GetVehicle().GetCurrentStatus().Number() {
			case gtfs.VehiclePosition_IN_TRANSIT_TO.Number():
				status = pb.StopStatus_IN_TRANSIT_TO
			case gtfs.VehiclePosition_STOPPED_AT.Number():
				status = pb.StopStatus_STOPPED_AT
			case gtfs.VehiclePosition_IN_TRANSIT_TO.Number():
				status = pb.StopStatus_IN_TRANSIT_TO
			}
			if entity.Vehicle != nil {
				v.out <- &pb.VehiclePositionEvent{
					AgencyId:     v.agency_id,
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
		resp.Body.Close()
	}
}

func (v *VehiclePositionsSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(v, operator)
	return operator
}

func (v *VehiclePositionsSource) Out() <-chan any {
	return v.out
}

func (v *VehiclePositionsSource) SetHTTPClient(client HTTPClient) {
	v.client = client
}
