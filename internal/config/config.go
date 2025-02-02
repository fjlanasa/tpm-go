package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Name string

// Connector

type ConnectorConfig struct {
	Name Name `yaml:"name"`
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
	Type       PipelineType `yaml:"type"`
	Sources    []Name       `yaml:"sources"`
	StateStore Name         `yaml:"state_store"`
	Sinks      []Name       `yaml:"sinks"`
}

type GraphConfig struct {
	Sources     map[Name]SourceConfig     `yaml:"sources"`
	StateStores map[Name]StateStoreConfig `yaml:"state_stores"`
	Connectors  map[Name]ConnectorConfig  `yaml:"connectors"`
	Sinks       map[Name]SinkConfig       `yaml:"sinks"`
	Pipelines   []PipelineConfig          `yaml:"pipelines"`
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
