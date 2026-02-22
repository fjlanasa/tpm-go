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
	"google.golang.org/protobuf/types/known/timestamppb"
)

type tripKey struct {
	routeID     string
	directionID string
	vehicleID   string
	tripID      string
}

func (k tripKey) String() string {
	return fmt.Sprintf("%s-%s-%s-%s", k.routeID, k.directionID, k.vehicleID, k.tripID)
}

type TravelTimeEventProcessor struct {
	in         chan any
	out        chan any
	tripStates statestore.StateStore
}

func NewTravelTimeEventProcessor(ctx context.Context, stateStore statestore.StateStore) *TravelTimeEventProcessor {
	var tripStates statestore.StateStore
	if stateStore == nil {
		tripStates = statestore.NewStateStore(ctx, config.StateStoreConfig{
			Type: config.InMemoryStateStoreType,
			InMemory: config.InMemoryStateStoreConfig{
				Expiry: time.Hour * 2,
			},
		})
	} else {
		tripStates = stateStore
	}

	flow := &TravelTimeEventProcessor{
		in:         make(chan any),
		out:        make(chan any),
		tripStates: tripStates,
	}
	go flow.doStream(ctx)
	return flow
}

func (f *TravelTimeEventProcessor) In() chan<- any {
	return f.in
}

func (f *TravelTimeEventProcessor) Out() <-chan any {
	return f.out
}

func (f *TravelTimeEventProcessor) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *TravelTimeEventProcessor) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *TravelTimeEventProcessor) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (f *TravelTimeEventProcessor) process(event *pb.StopEvent) {
	if event == nil || event.GetStopEventType() != pb.StopEvent_ARRIVAL {
		return
	}

	currentArrival := event

	key := tripKey{
		routeID:     event.GetAttributes().GetRouteId(),
		directionID: event.GetAttributes().GetDirectionId(),
		vehicleID:   event.GetAttributes().GetVehicleId(),
		tripID:      event.GetAttributes().GetTripId(),
	}

	previousArrival, found := f.tripStates.Get(key.String(), func() proto.Message {
		return &pb.StopEvent{}
	})

	if found {
		prev, ok := previousArrival.(*pb.StopEvent)
		if !ok || prev == nil {
			slog.Warn("travel time state type assertion failed", "key", key.String())
			_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)
			return
		}

		if prev.GetAttributes().GetStopId() != currentArrival.GetAttributes().GetStopId() {
			startTime := prev.GetAttributes().GetTimestamp().AsTime()
			endTime := currentArrival.GetAttributes().GetTimestamp().AsTime()
			travelSeconds := int32(endTime.Sub(startTime).Seconds())

			if travelSeconds <= 0 {
				slog.Warn("non-positive travel time, skipping",
					"travel_seconds", travelSeconds,
					"vehicle", event.GetAttributes().GetVehicleId(),
					"origin", prev.GetAttributes().GetStopId(),
					"destination", currentArrival.GetAttributes().GetStopId())
				_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)
				return
			}

			f.out <- &pb.TravelTimeEvent{
				Attributes: &pb.EventAttributes{
					AgencyId:          event.GetAttributes().GetAgencyId(),
					RouteId:           event.GetAttributes().GetRouteId(),
					TripId:            event.GetAttributes().GetTripId(),
					DirectionId:       event.GetAttributes().GetDirectionId(),
					VehicleId:         event.GetAttributes().GetVehicleId(),
					ServiceDate:       event.GetAttributes().GetServiceDate(),
					OriginStopId:      prev.GetAttributes().GetStopId(),
					DestinationStopId: currentArrival.GetAttributes().GetStopId(),
					Timestamp:         currentArrival.GetAttributes().GetTimestamp(),
				},
				StartTime:         timestamppb.New(startTime),
				EndTime:           timestamppb.New(endTime),
				TravelTimeSeconds: travelSeconds,
			}

			// Only update state when stop actually changed
			_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)
		}
		// Do NOT update state for duplicate arrivals at the same stop
		// to avoid inflated travel times
	} else {
		// First arrival for this trip key
		_ = f.tripStates.Set(key.String(), currentArrival, time.Hour)
	}
}

func (f *TravelTimeEventProcessor) doStream(ctx context.Context) {
	defer close(f.out)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-f.in:
			if !ok {
				return
			}
			if event == nil {
				continue
			}
			if stopEvent, ok := event.(*pb.StopEvent); ok {
				f.process(stopEvent)
			}
		}
	}
}
