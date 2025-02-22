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

func (c *GraphConfigYaml) Materialize() GraphConfig {
	connectors := []ConnectorConfig{}
	for connectorID := range c.Connectors {
		connectors = append(connectors, ConnectorConfig{ID: connectorID})
	}
	pipelines := []PipelineConfig{}
	for pipelineID, pipeline := range c.Pipelines {
		sources := []SourceConfig{}
		for _, sourceID := range pipeline.Sources {
			source, ok := c.Sources[sourceID]
			if !ok {
				panic(fmt.Sprintf("source %s not found", sourceID))
			}
			source.ID = sourceID
			sources = append(sources, source)
		}
		sinks := []SinkConfig{}
		for _, sinkID := range pipeline.Sinks {
			sink, ok := c.Sinks[sinkID]
			if !ok {
				panic(fmt.Sprintf("sink %s not found", sinkID))
			}
			sink.ID = sinkID
			sinks = append(sinks, sink)
		}
		stateStore, ok := c.StateStores[pipeline.StateStore]
		if !ok {
			panic(fmt.Sprintf("state store %s not found", pipeline.StateStore))
		}
		stateStore.ID = pipeline.StateStore
		pipelines = append(pipelines, PipelineConfig{
			ID:         pipelineID,
			Type:       pipeline.Type,
			Sources:    sources,
			StateStore: stateStore,
			Sinks:      sinks,
		})
	}
	return GraphConfig{
		Connectors: connectors,
		Pipelines:  pipelines,
	}
}
