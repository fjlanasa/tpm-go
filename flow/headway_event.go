package flow

import (
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type headwayStopKey struct {
	routeId     string
	stopId      string
	directionId uint32
}

type HeadwayEventState struct {
	lastVehicleId string
	lastTimestamp *timestamppb.Timestamp
}

type HeadwayEventFlow struct {
	in            chan any
	out           chan any
	headwayStates map[headwayStopKey]*HeadwayEventState
}

func NewHeadwayEventFlow() *HeadwayEventFlow {
	flow := &HeadwayEventFlow{
		in:            make(chan any),
		out:           make(chan any),
		headwayStates: make(map[headwayStopKey]*HeadwayEventState),
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

	state := f.headwayStates[key]
	if state == nil {
		// First arrival at this stop/route/direction
		f.headwayStates[key] = &HeadwayEventState{
			lastVehicleId: event.GetVehicleId(),
			lastTimestamp: event.GetStopTimestamp(),
		}
		return
	} else if state.lastVehicleId == event.GetVehicleId() {
		// Same vehicle, no headway
		return
	}

	// Calculate headway
	currentTime := event.GetStopTimestamp().AsTime()
	lastTime := state.lastTimestamp.AsTime()
	headwaySeconds := int32(currentTime.Sub(lastTime).Seconds())

	headwayEvent := &pb.HeadwayTimeEvent{
		EventId:             event.GetEventId() + "-headway",
		RouteId:             event.GetRouteId(),
		StopId:              event.GetStopId(),
		DirectionId:         event.GetDirectionId(),
		LeadingVehicleId:    state.lastVehicleId,
		FollowingVehicleId:  event.GetVehicleId(),
		Timestamp:           event.GetStopTimestamp(),
		HeadwayTrunkSeconds: headwaySeconds,
	}

	// Update state
	state.lastVehicleId = event.GetVehicleId()
	state.lastTimestamp = event.GetStopTimestamp()

	f.out <- headwayEvent
}

func (f *HeadwayEventFlow) doStream() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer close(f.out)

	for {
		select {
		case event, ok := <-f.in:
			if !ok {
				return
			}
			if stopEvent, ok := event.(*pb.StopEvent); ok {
				f.process(stopEvent)
			}
		case <-ticker.C:
			// Clean up old states
			now := time.Now()
			for key, state := range f.headwayStates {
				if now.Sub(state.lastTimestamp.AsTime()) > time.Hour {
					delete(f.headwayStates, key)
				}
			}
		}
	}
}
