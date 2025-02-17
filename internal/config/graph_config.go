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

type PipelineConfigYaml struct {
	ID         ID           `yaml:"id"`
	AgencyID   ID           `yaml:"agency_id"`
	Type       PipelineType `yaml:"type"`
	Sources    []ID         `yaml:"sources"`
	StateStore ID           `yaml:"state_store"`
	Sinks      []ID         `yaml:"sinks"`
}

type PipelineConfig struct {
	ID         ID
	AgencyID   ID
	Type       PipelineType
	Sources    []SourceConfig
	StateStore StateStoreConfig
	Sinks      []SinkConfig
}

type GraphConfigYaml struct {
	Sources     map[ID]SourceConfig       `yaml:"sources"`
	StateStores map[ID]StateStoreConfig   `yaml:"state_stores"`
	Connectors  map[ID]interface{}        `yaml:"connectors"`
	Sinks       map[ID]SinkConfig         `yaml:"sinks"`
	Pipelines   map[ID]PipelineConfigYaml `yaml:"pipelines"`
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

func ReadGraphConfig(path string) (GraphConfig, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return GraphConfig{}, err
	}
	var config GraphConfigYaml
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return GraphConfig{}, err
	}
	return config.Materialize(), nil
}
