package pipelines

import (
	"context"
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/state_stores"
	"github.com/reugn/go-streams"
)

type headwayStopKey struct {
	routeId     string
	stopId      string
	directionId uint32
}

func (k headwayStopKey) String() string {
	return fmt.Sprintf("%s-%d-%s", k.routeId, k.directionId, k.stopId)
}

type HeadwayEventFlow struct {
	in            chan any
	out           chan any
	headwayStates state_stores.StateStore[*pb.StopEvent]
}

func NewHeadwayEventFlow(ctx context.Context, stateStore ...state_stores.StateStore[*pb.StopEvent]) *HeadwayEventFlow {
	if len(stateStore) == 0 {
		stateStore = []state_stores.StateStore[*pb.StopEvent]{
			state_stores.NewStateStore(ctx, config.StateStoreConfig{
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour * 2,
				},
			}, func() *pb.StopEvent {
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
	if event == nil || event.EventType != pb.StopEvent_DEPARTURE {
		return
	}

	key := headwayStopKey{
		routeId:     event.GetRouteId(),
		stopId:      event.GetStopId(),
		directionId: event.GetDirectionId(),
	}

	state, found := f.headwayStates.Get(key.String())
	if !found {
		// First arrival at this stop/route/direction
		f.headwayStates.Set(key.String(), event, time.Hour)
		return
	}

	if state.GetVehicleId() == event.GetVehicleId() {
		// Same vehicle, no headway
		return
	}

	// Calculate headway
	currentTime := event.GetStopTimestamp().AsTime()
	lastTime := state.GetStopTimestamp().AsTime()
	headwaySeconds := int32(currentTime.Sub(lastTime).Seconds())

	headwayEvent := &pb.HeadwayTimeEvent{
		EventId:             event.GetEventId() + "-headway",
		RouteId:             event.GetRouteId(),
		StopId:              event.GetStopId(),
		DirectionId:         event.GetDirectionId(),
		LeadingVehicleId:    state.GetVehicleId(),
		FollowingVehicleId:  event.GetVehicleId(),
		Timestamp:           event.GetStopTimestamp(),
		HeadwayTrunkSeconds: headwaySeconds,
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
