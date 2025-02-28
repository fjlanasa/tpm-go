package config

import (
	"fmt"
)

// Connector

type ConnectorConfig struct {
	ID ID `yaml:"id"`
}

type GraphConfigYaml struct {
	Sources     map[ID]SourceConfig       `yaml:"sources"`
	StateStores map[ID]StateStoreConfig   `yaml:"state_stores"`
	Connectors  map[ID]interface{}        `yaml:"connectors"`
	Sinks       map[ID]SinkConfig         `yaml:"sinks"`
	Pipelines   map[ID]PipelineConfigYaml `yaml:"pipelines"`
	Outlet      *SinkConfig               `yaml:"outlet"`
}

type GraphConfig struct {
	Connectors []ConnectorConfig
	Pipelines  []PipelineConfig
}

func (c *GraphConfigYaml) Materialize() (GraphConfig, error) {
	connectors := []ConnectorConfig{}
	for connectorID := range c.Connectors {
		connectors = append(connectors, ConnectorConfig{ID: connectorID})
	}
	pipelines := []PipelineConfig{}
	for pipelineID, pipeline := range c.Pipelines {
		sources := []SourceConfig{}
		for _, sourceID := range pipeline.Sources {
			if _, ok := c.Connectors[sourceID]; ok {
				sources = append(sources, SourceConfig{
					ID:   sourceID,
					Type: SourceTypeConnector,
					Connector: ConnectorConfig{
						ID: sourceID,
					},
				})
			} else {
				source, ok := c.Sources[sourceID]
				if !ok {
					return GraphConfig{}, fmt.Errorf("source %s not found", sourceID)
				}
				source.ID = sourceID
				sources = append(sources, source)
			}
		}
		sinks := []SinkConfig{}
		for _, sinkID := range pipeline.Sinks {
			if _, ok := c.Connectors[sinkID]; ok {
				sinks = append(sinks, SinkConfig{
					ID:   sinkID,
					Type: SinkTypeConnector,
					Connector: ConnectorConfig{
						ID: sinkID,
					},
				})
			} else {
				sink, ok := c.Sinks[sinkID]
				if !ok {
					return GraphConfig{}, fmt.Errorf("sink %s not found", sinkID)
				}
				sink.ID = sinkID
				sinks = append(sinks, sink)
			}
		}
		stateStore, ok := c.StateStores[pipeline.StateStore]
		if ok {
			stateStore.ID = pipeline.StateStore
		}
		pipelines = append(pipelines, PipelineConfig{
			ID:         pipelineID,
			AgencyID:   pipeline.AgencyID,
			Type:       pipeline.Type,
			Sources:    sources,
			StateStore: &stateStore,
			Sinks:      sinks,
		})
	}
	return GraphConfig{
		Connectors: connectors,
		Pipelines:  pipelines,
	}, nil
}
