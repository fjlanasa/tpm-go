package vehicle_position_events

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/proto"
)

type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	return m.response, m.err
}

func createMockFeedMessage() []byte {
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
			Timestamp:           proto.Uint64(uint64(time.Now().Unix())),
		},
		Entity: []*gtfs.FeedEntity{
			{
				Id: proto.String("1"),
				Vehicle: &gtfs.VehiclePosition{
					Vehicle: &gtfs.VehicleDescriptor{
						Id: proto.String("v1"),
					},
					Trip: &gtfs.TripDescriptor{
						RouteId: proto.String("Red"),
					},
				},
			},
		},
	}
	data, _ := proto.Marshal(feed)
	return data
}

func TestVehiclePositionsSource(t *testing.T) {
	mockResponse := &http.Response{
		Body: io.NopCloser(bytes.NewReader(createMockFeedMessage())),
	}

	source := NewVehiclePositionsSource("TEST", "http://test.com", 100*time.Millisecond)
	source.SetHTTPClient(&mockHTTPClient{response: mockResponse})

	// Wait for first message
	select {
	case msg := <-source.Out():
		if vp, ok := msg.(*pb.VehiclePositionEvent); !ok {
			t.Error("expected VehiclePosition")
		} else if vp.GetVehicleId() != "v1" {
			t.Errorf("got vehicle ID %s, want v1", vp.GetVehicleId())
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}
