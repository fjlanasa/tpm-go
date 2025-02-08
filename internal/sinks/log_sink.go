package sinks

import (
	"context"
	"log/slog"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/reugn/go-streams"
)

type LogSink struct {
	streams.Sink
	logger *slog.Logger
	level  slog.Level
	in     chan any
	ctx    context.Context
}

func NewLogSink(ctx context.Context, cfg config.ConsoleSinkConfig) *LogSink {
	level := slog.LevelInfo
	if cfg.Level != "" {
		switch cfg.Level {
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}
	sink := &LogSink{
		ctx:    ctx,
		logger: slog.Default(),
		in:     make(chan any),
		level:  level,
	}
	go sink.doSink(ctx)
	return sink
}

func (s *LogSink) doSink(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.in:
			switch event := event.(type) {
			case *events.VehiclePositionEvent:
				s.logger.Log(ctx, s.level, "vehicle-position",
					"agency_id", event.GetAgencyId(),
					"route_id", event.GetTripId(),
					"direction_id", event.GetDirectionId(),
					"stop_id", event.GetStopId(),
					"latitude", event.GetLatitude(),
					"longitude", event.GetLongitude(),
				)
			case *events.StopEvent:
				s.logger.Log(ctx, s.level, "stop",
					"agency_id", event.GetAgencyId(),
					"route_id", event.GetRouteId(),
					"direction_id", event.GetDirectionId(),
					"stop_id", event.GetStopId(),
					"type", event.GetEventType(),
					"timestamp", event.GetTimestamp(),
				)
			case *events.DwellTimeEvent:
				s.logger.Log(ctx, s.level, "dwell",
					"agency_id", event.GetAgencyId(),
					"route_id", event.GetRouteId(),
					"direction_id", event.GetDirectionId(),
					"stop_id", event.GetStopId(),
					"dwell_time_seconds", event.GetDwellTimeSeconds(),
					"timestamp", event.GetTimestamp(),
				)
			case *events.HeadwayTimeEvent:
				s.logger.Log(ctx, s.level, "headway",
					"agency_id", event.GetAgencyId(),
					"route_id", event.GetRouteId(),
					"direction_id", event.GetDirectionId(),
					"stop_id", event.GetStopId(),
					"headway_time_seconds", event.GetHeadwayBranchSeconds(),
					"timestamp", event.GetTimestamp(),
				)
			case *events.TravelTimeEvent:
				s.logger.Log(ctx, s.level, "travel-time",
					"agency_id", event.GetAgencyId(),
					"route_id", event.GetRouteId(),
					"direction_id", event.GetDirectionId(),
					"origin_stop_id", event.GetFromStopId(),
					"destination_stop_id", event.GetToStopId(),
					"travel_time_seconds", event.GetTravelTimeSeconds(),
					"timestamp", event.GetTimestamp(),
				)
			}
		}
	}
}

func (s *LogSink) In() chan<- any {
	return s.in
}
