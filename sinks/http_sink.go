package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/event_server"
)

type HttpSink struct {
	server *event_server.EventServer
	in     chan any
}

func NewHttpSink(ctx context.Context, server *event_server.EventServer) *HttpSink {
	sink := &HttpSink{server: server, in: make(chan any)}
	go func() {
		defer close(sink.in)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sink.in:
				if !ok {
					return
				}
				switch v := event.(type) {

				case *events.VehiclePositionEvent:
					server.Broadcast(event_server.EventWrapper{
						AgencyId:    v.GetAgencyId(),
						RouteId:     v.GetRouteId(),
						DirectionId: v.GetDirectionId(),
						StopId:      v.GetStopId(),
						EventType:   "vehicle-position-event",
						Event:       v,
					})

				case *events.StopEvent:
					server.Broadcast(event_server.EventWrapper{
						AgencyId:    v.GetAgencyId(),
						RouteId:     v.GetRouteId(),
						DirectionId: v.GetDirectionId(),
						StopId:      v.GetStopId(),
						EventType:   "stop-event",
						Event:       v,
					})
				case *events.DwellTimeEvent:
					server.Broadcast(event_server.EventWrapper{
						AgencyId:    v.GetAgencyId(),
						RouteId:     v.GetRouteId(),
						DirectionId: v.GetDirectionId(),
						StopId:      v.GetStopId(),
						EventType:   "dwell-time-event",
						Event:       v,
					})
				case *events.HeadwayTimeEvent:
					server.Broadcast(event_server.EventWrapper{
						AgencyId:    v.GetAgencyId(),
						RouteId:     v.GetRouteId(),
						DirectionId: v.GetDirectionId(),
						StopId:      v.GetStopId(),
						EventType:   "headway-time-event",
						Event:       v,
					})
				case *events.TravelTimeEvent:
					server.Broadcast(event_server.EventWrapper{
						AgencyId:          v.GetAgencyId(),
						RouteId:           v.GetRouteId(),
						DirectionId:       v.GetDirectionId(),
						StopId:            v.GetParentStation(),
						OriginStopId:      v.GetFromStopId(),
						DestinationStopId: v.GetToStopId(),
						EventType:         "travel-time-event",
						Event:             v,
					})
				}
			}
		}
	}()
	return sink
}

func (hs *HttpSink) In() chan<- any {
	return hs.in
}
