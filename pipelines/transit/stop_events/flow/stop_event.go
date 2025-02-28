package stop_events

import (
	"context"
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/state_stores"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VehicleId string

type StopEventFlow struct {
	in            chan any
	out           chan any
	vehiclesState state_stores.StateStore
}

func NewStopEventFlow(ctx context.Context, state ...state_stores.StateStore) *StopEventFlow {
	var vehiclesState state_stores.StateStore
	if len(state) > 0 {
		vehiclesState = state[0]
	} else {
		vehiclesState = state_stores.NewStateStore(ctx, config.StateStoreConfig{
			Type: config.InMemoryStateStoreType,
			InMemory: config.InMemoryStateStoreConfig{
				Expiry: time.Hour * 2,
			},
		}, func() proto.Message {
			return &pb.VehiclePositionEvent{}
		})
	}

	flow := &StopEventFlow{
		in:            make(chan any),
		out:           make(chan any),
		vehiclesState: vehiclesState,
	}
	go flow.doStream(ctx)
	return flow
}

func (s *StopEventFlow) In() chan<- any {
	return s.in
}

func (s *StopEventFlow) Out() <-chan any {
	return s.out
}

func (s *StopEventFlow) Via(flow streams.Flow) streams.Flow {
	go s.transmit(flow)
	return flow
}

func (s *StopEventFlow) To(sink streams.Sink) {
	go s.transmit(sink)
}

func (s *StopEventFlow) transmit(inlet streams.Inlet) {
	for element := range s.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (s *StopEventFlow) makeStopEvent(vp *pb.VehiclePositionEvent, stopId string, eventType pb.StopEvent_EventType) *pb.StopEvent {
	if vp == nil {
		fmt.Println("Warning: nil VehiclePosition")
		return nil
	}

	vehicleID := vp.GetVehicleId()

	if vehicleID == "" || stopId == "" {
		fmt.Printf("Warning: missing required fields - vehicleID: %s, stopId: %s\n", vehicleID, stopId)
		return nil
	}
	return &pb.StopEvent{
		VehicleId:      vp.GetVehicleId(),
		StopId:         stopId,
		EventId:        fmt.Sprintf("%s-%s-%d", vp.GetVehicleId(), stopId, vp.GetTimestamp().AsTime().Unix()),
		VehicleLabel:   vp.GetVehicleLabel(),
		ServiceDate:    vp.GetServiceDate(),
		RouteId:        vp.GetRouteId(),
		TripId:         vp.GetTripId(),
		DirectionId:    vp.GetDirectionId(),
		StopSequence:   int32(vp.GetStopSequence()),
		MoveTimestamp:  timestamppb.New(vp.GetTimestamp().AsTime()),
		StopTimestamp:  timestamppb.New(vp.GetTimestamp().AsTime()),
		EventType:      eventType,
		AgencyId:       vp.GetAgencyId(),
		VehicleConsist: vp.GetVehicleConsist(),
		BranchRouteId:  vp.GetBranchRouteId(),
		TrunkRouteId:   vp.GetTrunkRouteId(),
		Direction:      vp.GetDirection(),
		ParentStation:  vp.GetParentStation(),
	}
}

func (s *StopEventFlow) process(event *pb.VehiclePositionEvent) {
	if event == nil {
		panic("nil VehiclePositionEvent")
	}
	if previousState, found := s.vehiclesState.Get(event.GetVehicleId()); found {
		previous := previousState.(*pb.VehiclePositionEvent)

		if previous.GetStopStatus() == event.GetStopStatus() && previous.GetStopId() == event.GetStopId() {
			return
		}
		switch event.GetStopStatus() {
		case pb.StopStatus_STOPPED_AT:
			if previous.GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO || previous.GetStopStatus() == pb.StopStatus_INCOMING_AT {
				// Arrival event
				s.out <- s.makeStopEvent(event, event.GetStopId(), pb.StopEvent_ARRIVAL)
			}
		case pb.StopStatus_IN_TRANSIT_TO:
			if previous.GetStopStatus() == pb.StopStatus_STOPPED_AT {
				// Simple departure event - vehicle was stopped and is now in transit
				s.out <- s.makeStopEvent(event, previous.GetStopId(), pb.StopEvent_DEPARTURE)
			} else if previous.GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO &&
				previous.GetStopId() != event.GetStopId() {
				// Vehicle changed destination while in transit
				// First complete previous stop events
				s.out <- s.makeStopEvent(previous, previous.GetStopId(), pb.StopEvent_ARRIVAL)
				s.out <- s.makeStopEvent(event, previous.GetStopId(), pb.StopEvent_DEPARTURE)
			}
			// Note: We don't generate events when a vehicle stays IN_TRANSIT_TO the same stop
		}
	}
	s.vehiclesState.Set(event.GetVehicleId(), event, time.Hour)
}

func (s *StopEventFlow) doStream(ctx context.Context) {
	defer close(s.out)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.in:
			if !ok {
				return
			}
			if vp, ok := event.(*pb.VehiclePositionEvent); ok {
				s.process(vp)
			}
		}
	}
}
