package flow

import (
	"fmt"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VehiclePositionState struct {
	vehicle   *gtfs.VehiclePosition
	timestamp int64
}

type StopEventFlow struct {
	in            chan any
	out           chan any
	vehiclesState map[string]*VehiclePositionState
}

func NewStopEventFlow() *StopEventFlow {
	flow := &StopEventFlow{
		in:            make(chan any),
		out:           make(chan any),
		vehiclesState: make(map[string]*VehiclePositionState),
	}
	go flow.doStream()
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

func (s *StopEventFlow) makeStopEvent(vp *gtfs.VehiclePosition, stopId string, eventType pb.StopEvent_EventType) *pb.StopEvent {
	if vp == nil {
		fmt.Println("Warning: nil VehiclePosition")
		return nil
	}

	vehicleID := vp.GetVehicle().GetId()

	if vehicleID == "" || stopId == "" {
		fmt.Printf("Warning: missing required fields - vehicleID: %s, stopId: %s\n", vehicleID, stopId)
		return nil
	}
	return &pb.StopEvent{
		VehicleId:     vp.GetVehicle().GetId(),
		StopId:        stopId,
		EventId:       fmt.Sprintf("%s-%s-%d", vp.GetVehicle().GetId(), stopId, vp.GetTimestamp()),
		VehicleLabel:  vp.GetVehicle().GetLabel(),
		ServiceDate:   vp.GetTrip().GetStartDate(),
		StartTime:     vp.GetTrip().GetStartTime(),
		RouteId:       vp.GetTrip().GetRouteId(),
		TripId:        vp.GetTrip().GetTripId(),
		DirectionId:   vp.GetTrip().GetDirectionId(),
		ParentStation: stopId,
		StopSequence:  int32(vp.GetCurrentStopSequence()),
		MoveTimestamp: timestamppb.New(time.Unix(int64(vp.GetTimestamp()), 0)),
		StopTimestamp: timestamppb.New(time.Unix(int64(vp.GetTimestamp()), 0)),
		EventType:     eventType,
	}
}

func (s *StopEventFlow) process(event *gtfs.VehiclePosition) {
	if previousState, found := s.vehiclesState[event.GetVehicle().GetId()]; found {
		previous := previousState.vehicle
		if previous.GetCurrentStatus() == event.GetCurrentStatus() && previous.GetStopId() == event.GetStopId() {
			return
		}
		switch event.GetCurrentStatus() {
		case gtfs.VehiclePosition_STOPPED_AT:
			if previous.GetCurrentStatus() == gtfs.VehiclePosition_IN_TRANSIT_TO || previous.GetCurrentStatus() == gtfs.VehiclePosition_INCOMING_AT {
				// Arrival event
				s.out <- s.makeStopEvent(event, event.GetStopId(), pb.StopEvent_ARRIVAL)
			}
		case gtfs.VehiclePosition_IN_TRANSIT_TO:
			if previous.GetCurrentStatus() == gtfs.VehiclePosition_STOPPED_AT {
				// Simple departure event - vehicle was stopped and is now in transit
				s.out <- s.makeStopEvent(event, previous.GetStopId(), pb.StopEvent_DEPARTURE)
			} else if previous.GetCurrentStatus() == gtfs.VehiclePosition_IN_TRANSIT_TO &&
				previous.GetStopId() != event.GetStopId() {
				// Vehicle changed destination while in transit
				// First complete previous stop events
				s.out <- s.makeStopEvent(previous, previous.GetStopId(), pb.StopEvent_ARRIVAL)
				s.out <- s.makeStopEvent(event, previous.GetStopId(), pb.StopEvent_DEPARTURE)
			}
			// Note: We don't generate events when a vehicle stays IN_TRANSIT_TO the same stop
		}
	}
	s.vehiclesState[event.GetVehicle().GetId()] = &VehiclePositionState{
		vehicle:   event,
		timestamp: int64(*event.Timestamp),
	}
}

func (s *StopEventFlow) removeStaleVehicles() {
	now := time.Now().Unix()
	for vehicleId, state := range s.vehiclesState {
		// If last event was more than an hour ago
		if now-state.timestamp > 3600 {
			delete(s.vehiclesState, vehicleId)
		}
	}
}
func (s *StopEventFlow) doStream() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer close(s.out)

	for {
		select {
		case event, ok := <-s.in: // ok is false if channel is closed, true if value was received
			if !ok {
				return
			}
			if vp, ok := event.(*gtfs.VehiclePosition); ok {
				s.process(vp)
			}
		case <-ticker.C:
			s.removeStaleVehicles()
		}
	}
}
