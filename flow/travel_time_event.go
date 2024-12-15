package flow

import (
	"fmt"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type tripKey struct {
	routeId     string
	directionId uint32
	vehicleId   string
	tripId      string
}

type stopArrival struct {
	stopId    string
	sequence  int32
	timestamp *timestamppb.Timestamp
}

type TravelTimeEventFlow struct {
	in         chan any
	out        chan any
	tripStates map[tripKey][]stopArrival
}

func NewTravelTimeEventFlow() *TravelTimeEventFlow {
	flow := &TravelTimeEventFlow{
		in:         make(chan any),
		out:        make(chan any),
		tripStates: make(map[tripKey][]stopArrival),
	}
	go flow.doStream()
	return flow
}

func (f *TravelTimeEventFlow) In() chan<- any {
	return f.in
}

func (f *TravelTimeEventFlow) Out() <-chan any {
	return f.out
}

func (f *TravelTimeEventFlow) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *TravelTimeEventFlow) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *TravelTimeEventFlow) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (f *TravelTimeEventFlow) process(event *pb.StopEvent) {
	if event == nil || event.EventType != pb.StopEvent_ARRIVAL {
		return
	}

	key := tripKey{
		routeId:     event.GetRouteId(),
		directionId: event.GetDirectionId(),
		vehicleId:   event.GetVehicleId(),
		tripId:      event.GetTripId(),
	}

	currentArrival := &stopArrival{
		stopId:    event.GetStopId(),
		sequence:  event.GetStopSequence(),
		timestamp: event.GetStopTimestamp(),
	}

	if previousArrivals, found := f.tripStates[key]; found {
		// Calculate travel time between stops
		for _, previousArrival := range previousArrivals {
			travelSeconds := int32(currentArrival.timestamp.AsTime().Sub(previousArrival.timestamp.AsTime()).Seconds())

			travelEvent := &pb.TravelTimeEvent{
				EventId:           fmt.Sprintf("%s-%s-%d-travel", event.GetVehicleId(), event.GetStopId(), event.GetStopTimestamp().AsTime().Unix()),
				RouteId:           event.GetRouteId(),
				TripId:            event.GetTripId(),
				DirectionId:       event.GetDirectionId(),
				VehicleId:         event.GetVehicleId(),
				FromStopId:        previousArrival.stopId,
				ToStopId:          currentArrival.stopId,
				TravelTimeSeconds: travelSeconds,
				Timestamp:         currentArrival.timestamp,
			}

			f.out <- travelEvent
		}
	}

	// Update state with current arrival
	f.tripStates[key] = append(f.tripStates[key], *currentArrival)
}

func (f *TravelTimeEventFlow) doStream() {
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
			for key, state := range f.tripStates {
				if now.Sub(state[0].timestamp.AsTime()) > time.Hour {
					delete(f.tripStates, key)
				}
			}
		}
	}
}
