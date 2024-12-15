package flow

import (
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type dwellStopKey struct {
	routeId     string
	stopId      string
	directionId uint32
	vehicleId   string
}

type DwellState struct {
	lastVehicleId string
	lastTimestamp *timestamppb.Timestamp
}

type DwellEventFlow struct {
	in          chan any
	out         chan any
	dwellStates map[dwellStopKey]*DwellState
}

func NewDwellEventFlow() *DwellEventFlow {
	flow := &DwellEventFlow{
		in:          make(chan any),
		out:         make(chan any),
		dwellStates: make(map[dwellStopKey]*DwellState),
	}

	go flow.doStream()
	return flow
}

func (d *DwellEventFlow) In() chan<- any {
	return d.in
}

func (d *DwellEventFlow) Out() <-chan any {
	return d.out
}

func (d *DwellEventFlow) Via(flow streams.Flow) streams.Flow {
	go d.transmit(flow)
	return flow
}

func (d *DwellEventFlow) To(sink streams.Sink) {
	go d.transmit(sink)
}

func (d *DwellEventFlow) transmit(inlet streams.Inlet) {
	for element := range d.Out() {
		inlet.In() <- element
	}
}

func (d *DwellEventFlow) process(event *pb.StopEvent) {
	if event == nil {
		return
	}

	key := dwellStopKey{
		routeId:     event.GetRouteId(),
		stopId:      event.GetStopId(),
		directionId: event.GetDirectionId(),
		vehicleId:   event.GetVehicleId(),
	}

	currentState, found := d.dwellStates[key]

	if found && event.GetEventType() == pb.StopEvent_DEPARTURE {
		// Calculate dwell time
		dwellSeconds := int32(event.GetStopTimestamp().AsTime().Sub(currentState.lastTimestamp.AsTime()).Seconds())

		dwellEvent := &pb.DwellTimeEvent{
			EventId:          fmt.Sprintf("%s-%s-%d-dwell", event.GetVehicleId(), event.GetStopId(), event.GetStopTimestamp().AsTime().Unix()),
			RouteId:          event.GetRouteId(),
			StopId:           event.GetStopId(),
			DirectionId:      event.GetDirectionId(),
			VehicleId:        event.GetVehicleId(),
			DwellTimeSeconds: dwellSeconds,
			Timestamp:        event.GetStopTimestamp(),
		}
		delete(d.dwellStates, key)
		d.out <- dwellEvent
	} else if event.GetEventType() == pb.StopEvent_ARRIVAL {
		d.dwellStates[key] = &DwellState{
			lastVehicleId: event.GetVehicleId(),
			lastTimestamp: timestamppb.New(event.GetStopTimestamp().AsTime()),
		}
	}
}

func (d *DwellEventFlow) doStream() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer close(d.out)

	for {
		select {
		case event, ok := <-d.in:
			if !ok {
				return
			}
			if vp, ok := event.(*pb.StopEvent); ok {
				d.process(vp)
			}
		case <-ticker.C:
			// Clean up old states
			now := time.Now()
			for key, state := range d.dwellStates {
				if now.Sub(state.lastTimestamp.AsTime()) > time.Hour {
					delete(d.dwellStates, key)
				}
			}
		}
	}
}
