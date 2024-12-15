package source

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
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

		fmt.Println("Received feed with", len(feed.Entity), "entities")
		for _, entity := range feed.Entity {
			if entity.Vehicle != nil {
				v.out <- entity.Vehicle
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
