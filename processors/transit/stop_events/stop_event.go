package processors

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

type VehicleId string

type StopEventProcessor struct {
	in            chan any
	out           chan any
	vehiclesState state_stores.StateStore
}

func NewStopEventProcessor(ctx context.Context, state ...state_stores.StateStore) *StopEventProcessor {
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

	flow := &StopEventProcessor{
		in:            make(chan any),
		out:           make(chan any),
		vehiclesState: vehiclesState,
	}
	go flow.doStream(ctx)
	return flow
}

func (s *StopEventProcessor) In() chan<- any {
	return s.in
}

func (s *StopEventProcessor) Out() <-chan any {
	return s.out
}

func (s *StopEventProcessor) Via(flow streams.Flow) streams.Flow {
	go s.transmit(flow)
	return flow
}

func (s *StopEventProcessor) To(sink streams.Sink) {
	go s.transmit(sink)
}

func (s *StopEventProcessor) transmit(inlet streams.Inlet) {
	for element := range s.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (s *StopEventProcessor) makeStopEvent(vp *pb.VehiclePositionEvent, stopId string, eventType pb.StopEvent_EventType) *pb.StopEvent {
	if vp == nil {
		fmt.Println("Warning: nil VehiclePosition")
		return nil
	}

	vehicleID := vp.GetAttributes().GetVehicleId()

	if vehicleID == "" || stopId == "" {
		fmt.Printf("Warning: missing required fields - vehicleID: %s, stopId: %s\n", vehicleID, stopId)
		return nil
	}
	return &pb.StopEvent{
		Attributes: &pb.EventAttributes{
			AgencyId:     vp.GetAttributes().GetAgencyId(),
			RouteId:      vp.GetAttributes().GetRouteId(),
			StopId:       stopId,
			DirectionId:  vp.GetAttributes().GetDirection(),
			StopSequence: int32(vp.GetAttributes().GetStopSequence()),
			TripId:       vp.GetAttributes().GetTripId(),
			ServiceDate:  vp.GetAttributes().GetServiceDate(),
			Timestamp:    vp.GetAttributes().GetTimestamp(),
			VehicleId:    vehicleID,
		},
		StopEventType: eventType,
	}
}

func (s *StopEventProcessor) process(event *pb.VehiclePositionEvent) {
	if event == nil {
		panic("nil VehiclePositionEvent")
	}
	if previousState, found := s.vehiclesState.Get(event.GetAttributes().GetVehicleId()); found {
		previous := previousState.(*pb.VehiclePositionEvent)

		if previous.GetAttributes().GetStopStatus() == event.GetAttributes().GetStopStatus() && previous.GetAttributes().GetStopId() == event.GetAttributes().GetStopId() {
			return
		}
		switch event.GetAttributes().GetStopStatus() {
		case pb.StopStatus_STOPPED_AT:
			if previous.GetAttributes().GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO || previous.GetAttributes().GetStopStatus() == pb.StopStatus_INCOMING_AT {
				// Arrival event
				s.out <- s.makeStopEvent(event, event.GetAttributes().GetStopId(), pb.StopEvent_ARRIVAL)
			}
		case pb.StopStatus_IN_TRANSIT_TO:
			if previous.GetAttributes().GetStopStatus() == pb.StopStatus_STOPPED_AT {
				// Simple departure event - vehicle was stopped and is now in transit
				s.out <- s.makeStopEvent(event, previous.GetAttributes().GetStopId(), pb.StopEvent_DEPARTURE)
			} else if previous.GetAttributes().GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO &&
				previous.GetAttributes().GetStopId() != event.GetAttributes().GetStopId() {
				// Vehicle changed destination while in transit
				// First complete previous stop events
				s.out <- s.makeStopEvent(previous, previous.GetAttributes().GetStopId(), pb.StopEvent_ARRIVAL)
				s.out <- s.makeStopEvent(event, previous.GetAttributes().GetStopId(), pb.StopEvent_DEPARTURE)
			}
			// Note: We don't generate events when a vehicle stays IN_TRANSIT_TO the same stop
		}
	}
	s.vehiclesState.Set(event.GetAttributes().GetVehicleId(), event, time.Hour)
}

func (s *StopEventProcessor) doStream(ctx context.Context) {
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
