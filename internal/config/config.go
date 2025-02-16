package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ID string

// Connector

type ConnectorConfig struct {
	ID ID `yaml:"id"`
}

type PipelineType string

const (
	PipelineTypeFeedMessage     PipelineType = "feed_message"
	PipelineTypeVehiclePosition PipelineType = "vehicle_position"
	PipelineTypeStopEvent       PipelineType = "stop_event"
	PipelineTypeDwellEvent      PipelineType = "dwell_event"
	PipelineTypeHeadwayEvent    PipelineType = "headway_event"
	PipelineTypeTravelTime      PipelineType = "travel_time"
)

// Pipeline

type PipelineConfig struct {
	ID         ID           `yaml:"id"`
	Type       PipelineType `yaml:"type"`
	Sources    []ID         `yaml:"sources"`
	StateStore ID           `yaml:"state_store"`
	Sinks      []ID         `yaml:"sinks"`
}

type MaterializedPipelineConfig struct {
	ID         ID
	Type       PipelineType
	Sources    []SourceConfig
	StateStore StateStoreConfig
	Sinks      []SinkConfig
}

type GraphConfig struct {
	Sources     map[ID]SourceConfig     `yaml:"sources"`
	StateStores map[ID]StateStoreConfig `yaml:"state_stores"`
	Connectors  map[ID]ConnectorConfig  `yaml:"connectors"`
	Sinks       map[ID]SinkConfig       `yaml:"sinks"`
	Pipelines   map[ID]PipelineConfig   `yaml:"pipelines"`
}

type MaterializedGraphConfig struct {
	Connectors map[ID]ConnectorConfig
	Pipelines  []MaterializedPipelineConfig
}

func (c *GraphConfig) Materialize() MaterializedGraphConfig {
	connectors := map[ID]ConnectorConfig{}
	for connectorID, connector := range c.Connectors {
		connector.ID = connectorID
		connectors[connectorID] = connector
	}
	pipelines := []MaterializedPipelineConfig{}
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
		pipelines = append(pipelines, MaterializedPipelineConfig{
			ID:         pipelineID,
			Type:       pipeline.Type,
			Sources:    sources,
			StateStore: stateStore,
			Sinks:      sinks,
		})
	}
	return MaterializedGraphConfig{
		Connectors: connectors,
		Pipelines:  pipelines,
	}
}

func ReadGraphConfig(path string) (MaterializedGraphConfig, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return MaterializedGraphConfig{}, err
	}
	var config GraphConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return MaterializedGraphConfig{}, err
	}
	return config.Materialize(), nil
}
