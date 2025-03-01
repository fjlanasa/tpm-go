package processors

import (
	"context"

	"github.com/fjlanasa/tpm-go/config"
	de "github.com/fjlanasa/tpm-go/processors/transit/dwell_events"
	fm "github.com/fjlanasa/tpm-go/processors/transit/feed_message_events"
	he "github.com/fjlanasa/tpm-go/processors/transit/headway_events"
	se "github.com/fjlanasa/tpm-go/processors/transit/stop_events"
	te "github.com/fjlanasa/tpm-go/processors/transit/travel_time_events"
	ve "github.com/fjlanasa/tpm-go/processors/transit/vehicle_position_events"
	"github.com/fjlanasa/tpm-go/state_stores"
	"github.com/reugn/go-streams"
)

type Processor interface {
	streams.Flow
}

func NewProcessor(ctx context.Context, agencyID config.ID, pipelineType config.PipelineType, stateStore state_stores.StateStore) Processor {
	switch pipelineType {
	case config.PipelineTypeFeedMessage:
		return fm.NewFeedMessageProcessor(ctx, agencyID)
	case config.PipelineTypeVehiclePosition:
		return ve.NewVehiclePositionEventProcessor(ctx)
	case config.PipelineTypeStopEvent:
		return se.NewStopEventProcessor(ctx, stateStore)
	case config.PipelineTypeDwellEvent:
		return de.NewDwellEventProcessor(ctx, stateStore)
	case config.PipelineTypeHeadwayEvent:
		return he.NewHeadwayEventProcessor(ctx, stateStore)
	case config.PipelineTypeTravelTime:
		return te.NewTravelTimeEventProcessor(ctx, stateStore)
	}
	return nil
}
