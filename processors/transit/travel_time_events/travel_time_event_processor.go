package processors

import (
	"context"
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/statestore"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type tripKey struct {
	routeID     string
	directionID string
	vehicleID   string
	tripID      string
}

func (k tripKey) String() string {
	return fmt.Sprintf("%s-%s-%s-%s", k.routeID, k.directionID, k.vehicleID, k.tripID)
}

type TravelTimeEventProcessor struct {
	in         chan any
	out        chan any
	tripStates statestore.StateStore
}

func NewTravelTimeEventProcessor(ctx context.Context, stateStore statestore.StateStore) *TravelTimeEventProcessor {
	var tripStates statestore.StateStore
	if stateStore == nil {
		tripStates = statestore.NewStateStore(ctx, config.StateStoreConfig{
			Type: config.InMemoryStateStoreType,
			InMemory: config.InMemoryStateStoreConfig{
				Expiry: time.Hour * 2,
			},
		})
	} else {
		tripStates = stateStore
	}

	flow := &TravelTimeEventProcessor{
		in:         make(chan any),
		out:        make(chan any),
		tripStates: tripStates,
	}
	go flow.doStream()
	return flow
}

func (f *TravelTimeEventProcessor) In() chan<- any {
	return f.in
}

func (f *TravelTimeEventProcessor) Out() <-chan any {
	return f.out
}

func (f *TravelTimeEventProcessor) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *TravelTimeEventProcessor) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *TravelTimeEventProcessor) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (f *TravelTimeEventProcessor) process(event *pb.StopEvent) {
	if event == nil || event.GetStopEventType() != pb.StopEvent_ARRIVAL {
		return
	}

	currentArrival := event

	key := tripKey{
		routeID:     event.GetAttributes().GetRouteId(),
		directionID: event.GetAttributes().GetDirectionId(),
		vehicleID:   event.GetAttributes().GetVehicleId(),
		tripID:      event.GetAttributes().GetTripId(),
	}

	previousArrival, found := f.tripStates.Get(key.String(), func() proto.Message {
		return &pb.StopEvent{}
	})
	if found {
		prev := previousArrival.(*pb.StopEvent)
		if prev.GetAttributes().GetStopId() != currentArrival.GetAttributes().GetStopId() {
			travelSeconds := int32(currentArrival.GetAttributes().GetTimestamp().AsTime().Sub(prev.GetAttributes().GetTimestamp().AsTime()).Seconds())

			f.out <- &pb.TravelTimeEvent{
				Attributes: &pb.EventAttributes{
					AgencyId:          event.GetAttributes().GetAgencyId(),
					RouteId:           event.GetAttributes().GetRouteId(),
					TripId:            event.GetAttributes().GetTripId(),
					DirectionId:       event.GetAttributes().GetDirectionId(),
					VehicleId:         event.GetAttributes().GetVehicleId(),
					OriginStopId:      prev.GetAttributes().GetStopId(),
					DestinationStopId: currentArrival.GetAttributes().GetStopId(),
					Timestamp:         currentArrival.GetAttributes().GetTimestamp(),
				},
				TravelTimeSeconds: travelSeconds,
			}
		}
	}

	_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)
}

func (f *TravelTimeEventProcessor) doStream() {
	defer close(f.out)

	for event := range f.in {
		if event == nil {
			continue
		}
		if stopEvent, ok := event.(*pb.StopEvent); ok {
			f.process(stopEvent)
		}
	}
}
