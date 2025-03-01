package pipelines

import (
	"context"
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/state_stores"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type tripKey struct {
	routeId     string
	directionId string
	vehicleId   string
	tripId      string
}

func (k tripKey) String() string {
	return fmt.Sprintf("%s-%s-%s-%s", k.routeId, k.directionId, k.vehicleId, k.tripId)
}

type TravelTimeEventFlow struct {
	in         chan any
	out        chan any
	tripStates state_stores.StateStore
}

func NewTravelTimeEventFlow(ctx context.Context, stateStore ...state_stores.StateStore) *TravelTimeEventFlow {
	if len(stateStore) == 0 {
		stateStore = []state_stores.StateStore{
			state_stores.NewStateStore(ctx, config.StateStoreConfig{
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour * 2,
				},
			}, func() proto.Message {
				return &pb.StopEvent{}
			}),
		}
	}

	flow := &TravelTimeEventFlow{
		in:         make(chan any),
		out:        make(chan any),
		tripStates: stateStore[0],
	}
	go flow.doStream()
	return flow
}

func (f *TravelTimeEventFlow) In() chan<- any {
	return f.in
}

func (f *TravelTimeEventFlow) Out() <-chan any {
	return f.out
}

func (f *TravelTimeEventFlow) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *TravelTimeEventFlow) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *TravelTimeEventFlow) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (f *TravelTimeEventFlow) process(event *pb.StopEvent) {
	if event == nil || event.GetStopEventType() != pb.StopEvent_ARRIVAL {
		return
	}

	currentArrival := event

	key := tripKey{
		routeId:     event.GetAttributes().GetRouteId(),
		directionId: event.GetAttributes().GetDirectionId(),
		vehicleId:   event.GetAttributes().GetVehicleId(),
		tripId:      event.GetAttributes().GetTripId(),
	}

	previousArrival, found := f.tripStates.Get(key.String())
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

	f.tripStates.Set(key.String(), currentArrival, time.Hour)
}

func (f *TravelTimeEventFlow) doStream() {
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
