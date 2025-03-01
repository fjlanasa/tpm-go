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

type headwayStopKey struct {
	routeId     string
	stopId      string
	directionId string
}

func (k headwayStopKey) String() string {
	return fmt.Sprintf("%s-%s-%s", k.routeId, k.directionId, k.stopId)
}

type HeadwayEventFlow struct {
	in            chan any
	out           chan any
	headwayStates state_stores.StateStore
}

func NewHeadwayEventFlow(ctx context.Context, stateStore ...state_stores.StateStore) *HeadwayEventFlow {
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
	flow := &HeadwayEventFlow{
		in:            make(chan any),
		out:           make(chan any),
		headwayStates: stateStore[0],
	}
	go flow.doStream()
	return flow
}

func (f *HeadwayEventFlow) In() chan<- any {
	return f.in
}

func (f *HeadwayEventFlow) Out() <-chan any {
	return f.out
}

func (f *HeadwayEventFlow) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *HeadwayEventFlow) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *HeadwayEventFlow) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (f *HeadwayEventFlow) process(event *pb.StopEvent) {
	if event == nil || event.GetStopEventType() != pb.StopEvent_DEPARTURE {
		return
	}

	key := headwayStopKey{
		routeId:     event.GetAttributes().GetRouteId(),
		stopId:      event.GetAttributes().GetStopId(),
		directionId: event.GetAttributes().GetDirectionId(),
	}

	state, found := f.headwayStates.Get(key.String())
	// this should be a pb.StopEvent
	stopEvent := state.(*pb.StopEvent)
	if !found {
		// First arrival at this stop/route/direction
		f.headwayStates.Set(key.String(), event, time.Hour)
		return
	}

	if stopEvent.GetAttributes().GetVehicleId() == event.GetAttributes().GetVehicleId() {
		// Same vehicle, no headway
		return
	}

	// Calculate headway
	currentTime := event.GetAttributes().GetTimestamp().AsTime()
	lastTime := stopEvent.GetAttributes().GetTimestamp().AsTime()
	headwaySeconds := int32(currentTime.Sub(lastTime).Seconds())

	headwayEvent := &pb.HeadwayTimeEvent{
		Attributes: &pb.EventAttributes{
			AgencyId:    event.GetAttributes().GetAgencyId(),
			RouteId:     event.GetAttributes().GetRouteId(),
			StopId:      event.GetAttributes().GetStopId(),
			DirectionId: event.GetAttributes().GetDirectionId(),
			ServiceDate: event.GetAttributes().GetServiceDate(),
			Timestamp:   event.GetAttributes().GetTimestamp(),
			VehicleId:   event.GetAttributes().GetVehicleId(),
		},
		LeadingVehicleId:   stopEvent.GetAttributes().GetVehicleId(),
		FollowingVehicleId: event.GetAttributes().GetVehicleId(),
		HeadwaySeconds:     headwaySeconds,
	}

	// Update state
	f.headwayStates.Set(key.String(), event, time.Hour)

	f.out <- headwayEvent
}

func (f *HeadwayEventFlow) doStream() {
	defer close(f.out)

	for event := range f.in {
		if stopEvent, ok := event.(*pb.StopEvent); ok {
			f.process(stopEvent)
		}
	}
}
