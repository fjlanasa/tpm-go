package graphs

import (
	"context"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/pipelines"
	"google.golang.org/protobuf/proto"
)

type Graph struct {
	pipelines []pipelines.Pipeline
	outlet    *chan any
}

type GraphOption func(*Graph, *map[config.ID]chan any, *config.GraphConfig)

func WithOutlet(outlet *chan any) GraphOption {
	return func(g *Graph, connectors *map[config.ID]chan any, cfg *config.GraphConfig) {
		if outlet == nil {
			return
		}
		g.outlet = outlet
		(*connectors)["outlet"] = *outlet

		// Add outlet sink to each pipeline
		for i := range cfg.Pipelines {
			cfg.Pipelines[i].Sinks = append(cfg.Pipelines[i].Sinks, config.SinkConfig{
				ID:   config.ID("outlet"),
				Type: config.SinkTypeConnector,
				Connector: config.ConnectorConfig{
					ID: "outlet",
				},
			})
		}
	}
}

func NewGraph(ctx context.Context, cfg config.GraphConfig, opts ...GraphOption) (*Graph, error) {
	graph := &Graph{pipelines: []pipelines.Pipeline{}}
	connectors := make(map[config.ID]chan any)

	// Apply options
	for _, opt := range opts {
		opt(graph, &connectors, &cfg)
	}

	// Set up remaining connectors
	for _, connector := range cfg.Connectors {
		connectors[connector.ID] = make(chan any)
	}

	for _, pipelineConfig := range cfg.Pipelines {
		switch pipelineConfig.Type {
		case config.PipelineTypeFeedMessage:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &gtfs.FeedMessage{} },
				func() proto.Message { return &pb.FeedMessageEvent{} },
				connectors,
			))
		case config.PipelineTypeVehiclePosition:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &pb.FeedMessageEvent{} },
				func() proto.Message { return &pb.VehiclePositionEvent{} },
				connectors,
			))
		case config.PipelineTypeStopEvent:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &pb.VehiclePositionEvent{} },
				func() proto.Message { return &pb.StopEvent{} },
				connectors,
			))
		case config.PipelineTypeDwellEvent:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &pb.StopEvent{} },
				func() proto.Message { return &pb.DwellTimeEvent{} },
				connectors,
			))
		case config.PipelineTypeHeadwayEvent:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &pb.StopEvent{} },
				func() proto.Message { return &pb.HeadwayTimeEvent{} },
				connectors,
			))
		case config.PipelineTypeTravelTime:
			graph.pipelines = append(graph.pipelines, *pipelines.NewPipeline(
				ctx,
				pipelineConfig,
				func() proto.Message { return &pb.StopEvent{} },
				func() proto.Message { return &pb.TravelTimeEvent{} },
				connectors,
			))
		}
	}
	return graph, nil
}

func (g *Graph) Out() *chan any {
	return g.outlet
}

func (g *Graph) Run() {
	for _, pipeline := range g.pipelines {
		go pipeline.Run()
	}
}
