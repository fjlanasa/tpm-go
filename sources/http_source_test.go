package sources

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/fjlanasa/tpm-go/config"
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

	source, err := NewHttpSource(context.Background(), config.HTTPSourceConfig{
		URL:      "http://test.com",
		Interval: "100ms",
	}, &mockHTTPClient{response: mockResponse})
	if err != nil {
		t.Fatalf("failed to create http source: %v", err)
	}

	// Wait for first message
	select {
	case msg := <-source.Out():
		data := msg.([]byte)
		feed := &gtfs.FeedMessage{}
		err := proto.Unmarshal(data, feed)
		if err != nil {
			t.Errorf("failed to unmarshal feed message: %v", err)
		}
		if feed.GetHeader().GetGtfsRealtimeVersion() != "2.0" {
			t.Errorf("expected version 2.0, got %s", feed.GetHeader().GetGtfsRealtimeVersion())
		}
		if len(feed.GetEntity()) != 1 {
			t.Errorf("expected 1 entity, got %d", len(feed.GetEntity()))
		}
		if feed.GetEntity()[0].GetVehicle().GetVehicle().GetId() != "v1" {
			t.Errorf("expected vehicle id v1, got %s", feed.GetEntity()[0].GetVehicle().GetVehicle().GetId())
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}
