package config

import (
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
	PipelineTypeVehiclePosition PipelineType = "vehicle_position"
	PipelineTypeStopEvent       PipelineType = "stop_event"
	PipelineTypeDwellEvent      PipelineType = "dwell_event"
	PipelineTypeHeadwayEvent    PipelineType = "headway_event"
	PipelineTypeTravelTime      PipelineType = "travel_time"
)

// Pipeline

type PipelineConfig struct {
	ID         string       `yaml:"id"`
	Type       PipelineType `yaml:"type"`
	Sources    []ID         `yaml:"sources"`
	StateStore ID           `yaml:"state_store"`
	Sinks      []ID         `yaml:"sinks"`
}

type GraphConfig struct {
	Sources     map[ID]SourceConfig     `yaml:"sources"`
	StateStores map[ID]StateStoreConfig `yaml:"state_stores"`
	Connectors  map[ID]ConnectorConfig  `yaml:"connectors"`
	Sinks       map[ID]SinkConfig       `yaml:"sinks"`
	Pipelines   []PipelineConfig        `yaml:"pipelines"`
}

func ReadGraphConfig(path string) (*GraphConfig, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config GraphConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
