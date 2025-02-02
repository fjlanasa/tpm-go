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
			switch entity.Vehicle.CurrentStatus.Number() {
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
					EventId:      *entity.Id,
					VehicleId:    *entity.Id,
					VehicleLabel: *entity.Vehicle.Vehicle.Label,
					RouteId:      *entity.Vehicle.Trip.RouteId,
					TripId:       *entity.Vehicle.Trip.TripId,
					DirectionId:  uint32(*entity.Vehicle.Trip.DirectionId),
					StopId:       *entity.Vehicle.StopId,
					StopStatus:   status,
					StopSequence: int32(*entity.Vehicle.CurrentStopSequence),
					Latitude:     float64(*entity.Vehicle.Position.Latitude),
					Longitude:    float64(*entity.Vehicle.Position.Longitude),
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
