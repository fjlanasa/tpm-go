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

type DwellStopKey struct {
	agencyID    string
	routeID     string
	stopID      string
	directionID string
	vehicleID   string
}

func NewDwellStopKey(stopEvent *pb.StopEvent) DwellStopKey {
	return DwellStopKey{
		agencyID:    stopEvent.GetAttributes().GetAgencyId(),
		routeID:     stopEvent.GetAttributes().GetRouteId(),
		stopID:      stopEvent.GetAttributes().GetStopId(),
		directionID: stopEvent.GetAttributes().GetDirectionId(),
		vehicleID:   stopEvent.GetAttributes().GetVehicleId(),
	}
}

func (k *DwellStopKey) String() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s", k.agencyID, k.routeID, k.stopID, k.directionID, k.vehicleID)
}

type DwellEventProcessor struct {
	in         chan any
	out        chan any
	stateStore statestore.StateStore
}

func NewDwellEventProcessor(ctx context.Context, stateStore statestore.StateStore) *DwellEventProcessor {
	var dwellStates statestore.StateStore
	if stateStore == nil {
		dwellStates = statestore.NewStateStore(ctx, config.StateStoreConfig{
			Type: config.InMemoryStateStoreType,
			InMemory: config.InMemoryStateStoreConfig{
				Expiry: time.Hour,
			},
		})
	} else {
		dwellStates = stateStore
	}

	flow := &DwellEventProcessor{
		in:         make(chan any),
		out:        make(chan any),
		stateStore: dwellStates,
	}

	go flow.doStream()
	return flow
}

func (d *DwellEventProcessor) In() chan<- any {
	return d.in
}

func (d *DwellEventProcessor) Out() <-chan any {
	return d.out
}

func (d *DwellEventProcessor) Via(flow streams.Flow) streams.Flow {
	go d.transmit(flow)
	return flow
}

func (d *DwellEventProcessor) To(sink streams.Sink) {
	go d.transmit(sink)
}

func (d *DwellEventProcessor) transmit(inlet streams.Inlet) {
	for element := range d.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (d *DwellEventProcessor) process(event *pb.StopEvent) {
	if event == nil {
		return
	}

	key := NewDwellStopKey(event)

	currentState, found := d.stateStore.Get(key.String(), func() proto.Message {
		return &pb.StopEvent{}
	})
	if found && event.GetStopEventType() == pb.StopEvent_DEPARTURE {
		// Calculate dwell time
		dwellSeconds := int32(event.GetAttributes().GetTimestamp().AsTime().Sub(currentState.(*pb.StopEvent).GetAttributes().GetTimestamp().AsTime()).Seconds())

		dwellEvent := &pb.DwellTimeEvent{
			Attributes: &pb.EventAttributes{
				RouteId:     event.GetAttributes().GetRouteId(),
				StopId:      event.GetAttributes().GetStopId(),
				DirectionId: event.GetAttributes().GetDirectionId(),
				VehicleId:   event.GetAttributes().GetVehicleId(),
				Timestamp:   event.GetAttributes().GetTimestamp(),
			},
			DwellTimeSeconds: dwellSeconds,
		}
		d.stateStore.Delete(key.String())
		d.out <- dwellEvent
	} else if event.GetStopEventType() == pb.StopEvent_ARRIVAL {
		_ = d.stateStore.Set(key.String(), event, time.Hour)
	}
}

func (d *DwellEventProcessor) doStream() {
	defer close(d.out)

	for event := range d.in {
		if vp, ok := event.(*pb.StopEvent); ok {
			d.process(vp)
		}
	}
}
