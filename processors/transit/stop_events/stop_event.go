package processors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/statestore"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type VehicleID string

type StopEventProcessor struct {
	in            chan any
	out           chan any
	vehiclesState statestore.StateStore
}

func NewStopEventProcessor(ctx context.Context, stateStore statestore.StateStore) *StopEventProcessor {
	var vehiclesState statestore.StateStore
	if stateStore != nil {
		slog.Debug("Using provided state store")
		vehiclesState = stateStore
	} else {
		slog.Debug("Using default state store")
		vehiclesState = statestore.NewStateStore(ctx, config.StateStoreConfig{
			Type: config.InMemoryStateStoreType,
			InMemory: config.InMemoryStateStoreConfig{
				Expiry: time.Hour * 2,
			},
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

func (s *StopEventProcessor) makeStopEvent(vp *pb.VehiclePositionEvent, stopID string, eventType pb.StopEvent_EventType) *pb.StopEvent {
	if vp == nil {
		slog.Warn("nil VehiclePosition in makeStopEvent")
		return nil
	}

	vehicleID := vp.GetAttributes().GetVehicleId()

	if vehicleID == "" || stopID == "" {
		slog.Warn("missing required fields in makeStopEvent", "vehicleID", vehicleID, "stopID", stopID)
		return nil
	}
	return &pb.StopEvent{
		Attributes: &pb.EventAttributes{
			AgencyId:     vp.GetAttributes().GetAgencyId(),
			RouteId:      vp.GetAttributes().GetRouteId(),
			StopId:       stopID,
			DirectionId:  vp.GetAttributes().GetDirectionId(),
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
	if s == nil {
		slog.Error("StopEventProcessor is nil")
		return
	}

	if s.vehiclesState == nil {
		slog.Error("vehiclesState is nil")
		return
	}

	if event == nil {
		slog.Error("nil VehiclePositionEvent")
		return
	}

	attrs := event.GetAttributes()
	if attrs == nil {
		slog.Warn("nil Attributes in event")
		return
	}

	vehicleID := attrs.GetVehicleId()
	if vehicleID == "" {
		slog.Warn("empty vehicle ID in event")
		return
	}

	// Create a new message in a separate step to ensure it's valid
	newMsg := func() proto.Message {
		return &pb.VehiclePositionEvent{}
	}

	previousState, found := s.vehiclesState.Get(vehicleID, newMsg)
	if previousState == nil {
		slog.Warn("Got nil previousState from state store", "vehicleID", vehicleID)
		return
	}

	// Store current event first to ensure we don't lose state
	if err := s.vehiclesState.Set(vehicleID, event, time.Hour); err != nil {
		slog.Warn("Failed to store state",
			"vehicleID", vehicleID,
			"error", err)
		return
	}

	if !found {
		return
	}

	previous, ok := previousState.(*pb.VehiclePositionEvent)
	if !ok || previous == nil {
		slog.Warn("previous state conversion failed", "vehicleID", vehicleID)
		return
	}

	prevAttrs := previous.GetAttributes()
	if prevAttrs == nil {
		slog.Warn("previous state has nil attributes", "vehicleID", vehicleID)
		return
	}

	// Only proceed if there's actually a change in status or stop
	if prevAttrs.GetStopStatus() == attrs.GetStopStatus() &&
		prevAttrs.GetStopId() == attrs.GetStopId() {
		return
	}

	switch attrs.GetStopStatus() {
	case pb.StopStatus_STOPPED_AT:
		if prevAttrs.GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO || prevAttrs.GetStopStatus() == pb.StopStatus_INCOMING_AT {
			s.out <- s.makeStopEvent(event, attrs.GetStopId(), pb.StopEvent_ARRIVAL)
		} else if prevAttrs.GetStopStatus() == pb.StopStatus_STOPPED_AT &&
			prevAttrs.GetStopId() != attrs.GetStopId() {
			// Jumped from STOPPED_AT one stop to STOPPED_AT another (skipped transit phase)
			s.out <- s.makeStopEvent(event, prevAttrs.GetStopId(), pb.StopEvent_DEPARTURE)
			s.out <- s.makeStopEvent(event, attrs.GetStopId(), pb.StopEvent_ARRIVAL)
		}
	case pb.StopStatus_IN_TRANSIT_TO:
		if prevAttrs.GetStopStatus() == pb.StopStatus_STOPPED_AT {
			s.out <- s.makeStopEvent(event, prevAttrs.GetStopId(), pb.StopEvent_DEPARTURE)
		} else if (prevAttrs.GetStopStatus() == pb.StopStatus_IN_TRANSIT_TO ||
			prevAttrs.GetStopStatus() == pb.StopStatus_INCOMING_AT) &&
			prevAttrs.GetStopId() != attrs.GetStopId() {
			// Stop changed while in transit or incoming — synthesize arrival + departure
			s.out <- s.makeStopEvent(event, prevAttrs.GetStopId(), pb.StopEvent_ARRIVAL)
			s.out <- s.makeStopEvent(event, prevAttrs.GetStopId(), pb.StopEvent_DEPARTURE)
		}
	}
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
			} else {
				slog.Warn("Received non-VehiclePositionEvent", "type", fmt.Sprintf("%T", event))
			}
		}
	}
}
