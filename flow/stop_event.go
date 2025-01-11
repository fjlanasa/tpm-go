package flow

import (
	"fmt"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TimestampedVehiclePosition struct {
	vehicle   *gtfs.VehiclePosition
	timestamp int64
}

type VehiclePositionStateManager interface {
	Get(vehicleID string) (*TimestampedVehiclePosition, bool)
	Set(vehicleID string, state *TimestampedVehiclePosition)
	GetAndSet(vehicleID string, state *TimestampedVehiclePosition) (*TimestampedVehiclePosition, *TimestampedVehiclePosition)
	Remove(vehicleID string)
	RemoveStale()
}

type InMemoryVehiclePositionStateManager struct {
	maxAge        time.Duration
	vehiclesState map[string]*TimestampedVehiclePosition
}

func (v *InMemoryVehiclePositionStateManager) Get(vehicleID string) (*TimestampedVehiclePosition, bool) {
	state, ok := v.vehiclesState[vehicleID]
	return state, ok
}

func (v *InMemoryVehiclePositionStateManager) Set(vehicleID string, state *TimestampedVehiclePosition) {
	v.vehiclesState[vehicleID] = state
}

func (v *InMemoryVehiclePositionStateManager) Remove(vehicleID string) {
	delete(v.vehiclesState, vehicleID)
}

func (v *InMemoryVehiclePositionStateManager) RemoveStale() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		v.RemoveStale()
	}
}

func (v *InMemoryVehiclePositionStateManager) GetAndSet(vehicleID string, state *TimestampedVehiclePosition) (*TimestampedVehiclePosition, *TimestampedVehiclePosition) {
	previous, _ := v.Get(vehicleID)
	v.Set(vehicleID, state)
	return previous, state
}

func NewInMemoryVehiclePositionStateManager(maxAgeArg ...time.Duration) *InMemoryVehiclePositionStateManager {
	var maxAge time.Duration
	if len(maxAgeArg) > 0 {
		maxAge = maxAgeArg[0]
	} else {
		maxAge = 1 * time.Hour
	}
	manager := &InMemoryVehiclePositionStateManager{
		maxAge:        maxAge,
		vehiclesState: make(map[string]*TimestampedVehiclePosition),
	}

	go manager.RemoveStale()
	return manager
}

type StopEventFlow struct {
	in            chan any
	out           chan any
	vehiclesState VehiclePositionStateManager
}

func NewStopEventFlow(state ...VehiclePositionStateManager) *StopEventFlow {
	var vehiclesState VehiclePositionStateManager
	if len(state) > 0 {
		vehiclesState = state[0]
	} else {
		vehiclesState = NewInMemoryVehiclePositionStateManager()
	}

	flow := &StopEventFlow{
		in:            make(chan any),
		out:           make(chan any),
		vehiclesState: vehiclesState,
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
	if previousState, _ := s.vehiclesState.GetAndSet(event.GetVehicle().GetId(), &TimestampedVehiclePosition{
		vehicle:   event,
		timestamp: int64(*event.Timestamp),
	}); previousState != nil {
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
}

func (s *StopEventFlow) doStream() {
	defer close(s.out)

	for event := range s.in {
		if vp, ok := event.(*gtfs.VehiclePosition); ok {
			s.process(vp)
		}
	}
}
