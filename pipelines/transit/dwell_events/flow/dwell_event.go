package dwell_events

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

type DwellStopKey struct {
	agencyId    string
	routeId     string
	stopId      string
	directionId uint32
	vehicleId   string
}

func NewDwellStopKey(stopEvent *pb.StopEvent) DwellStopKey {
	return DwellStopKey{
		agencyId:    stopEvent.GetAgencyId(),
		routeId:     stopEvent.GetRouteId(),
		stopId:      stopEvent.GetStopId(),
		directionId: stopEvent.GetDirectionId(),
		vehicleId:   stopEvent.GetVehicleId(),
	}
}

func (k *DwellStopKey) String() string {
	return fmt.Sprintf("%s-%s-%s-%d-%s", k.agencyId, k.routeId, k.stopId, k.directionId, k.vehicleId)
}

type DwellEventFlow struct {
	in         chan any
	out        chan any
	stateStore state_stores.StateStore
}

func NewDwellEventFlow(ctx context.Context, stateStore ...state_stores.StateStore) *DwellEventFlow {
	if len(stateStore) == 0 {
		stateStore = []state_stores.StateStore{
			state_stores.NewStateStore(ctx, config.StateStoreConfig{
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour,
				},
			}, func() proto.Message {
				return &pb.StopEvent{}
			}),
		}
	}

	flow := &DwellEventFlow{
		in:         make(chan any),
		out:        make(chan any),
		stateStore: stateStore[0],
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

	key := NewDwellStopKey(event)

	currentState, found := d.stateStore.Get(key.String())
	if found && event.GetEventType() == pb.StopEvent_DEPARTURE {
		// Calculate dwell time
		dwellSeconds := int32(event.GetStopTimestamp().AsTime().Sub(currentState.(*pb.StopEvent).GetStopTimestamp().AsTime()).Seconds())

		dwellEvent := &pb.DwellTimeEvent{
			EventId:          fmt.Sprintf("%s-%s-%d-dwell", event.GetVehicleId(), event.GetStopId(), event.GetStopTimestamp().AsTime().Unix()),
			RouteId:          event.GetRouteId(),
			StopId:           event.GetStopId(),
			DirectionId:      event.GetDirectionId(),
			VehicleId:        event.GetVehicleId(),
			DwellTimeSeconds: dwellSeconds,
			Timestamp:        event.GetStopTimestamp(),
		}
		d.stateStore.Delete(key.String())
		d.out <- dwellEvent
	} else if event.GetEventType() == pb.StopEvent_ARRIVAL {
		d.stateStore.Set(key.String(), event, time.Hour)
	}
}

func (d *DwellEventFlow) doStream() {
	defer close(d.out)

	for event := range d.in {
		if vp, ok := event.(*pb.StopEvent); ok {
			d.process(vp)
		}
	}
}
