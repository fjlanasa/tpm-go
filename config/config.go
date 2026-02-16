package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ID string

type ConfigYaml struct {
	Graph *GraphConfig `yaml:"graph"`
}

type Config struct {
	Graph *GraphConfig
}

func ReadConfig(path string) (*Config, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config ConfigYaml
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	if config.Graph != nil {
		if err := config.Graph.Validate(); err != nil {
			return nil, fmt.Errorf("config validation: %w", err)
		}
	}
	return &Config{
		Graph: config.Graph,
	}, nil
}

// validPipelineTypes is the set of recognized pipeline types.
var validPipelineTypes = map[PipelineType]bool{
	PipelineTypeFeedMessage:     true,
	PipelineTypeVehiclePosition: true,
	PipelineTypeStopEvent:       true,
	PipelineTypeDwellEvent:      true,
	PipelineTypeHeadwayEvent:    true,
	PipelineTypeTravelTime:      true,
}

// Validate checks that the GraphConfig is internally consistent:
// - HTTP sources have a URL and valid interval
// - Pipelines reference valid types
// - Pipeline source/sink references exist in config (including connectors)
func (g *GraphConfig) Validate() error {
	// Build the set of known source and sink IDs (explicit + connectors)
	knownSources := make(map[ID]bool, len(g.Sources)+len(g.Connectors))
	for id := range g.Sources {
		knownSources[id] = true
	}
	for id := range g.Connectors {
		knownSources[id] = true
	}

	knownSinks := make(map[ID]bool, len(g.Sinks)+len(g.Connectors))
	for id := range g.Sinks {
		knownSinks[id] = true
	}
	for id := range g.Connectors {
		knownSinks[id] = true
	}

	// Validate HTTP source configs
	for id, src := range g.Sources {
		if src.Type == SourceTypeHTTP {
			if src.HTTP.URL == "" {
				return fmt.Errorf("source %q: http url is required", id)
			}
			if src.HTTP.Interval == "" {
				return fmt.Errorf("source %q: http interval is required", id)
			}
			if _, err := time.ParseDuration(src.HTTP.Interval); err != nil {
				return fmt.Errorf("source %q: invalid http interval %q: %w", id, src.HTTP.Interval, err)
			}
		}
	}

	// Validate pipelines
	for id, p := range g.Pipelines {
		if !validPipelineTypes[p.Type] {
			return fmt.Errorf("pipeline %q: invalid type %q", id, p.Type)
		}
		if len(p.Sources) == 0 {
			return fmt.Errorf("pipeline %q: at least one source is required", id)
		}
		if len(p.Sinks) == 0 {
			return fmt.Errorf("pipeline %q: at least one sink is required", id)
		}
		for _, srcID := range p.Sources {
			if !knownSources[srcID] {
				return fmt.Errorf("pipeline %q: references unknown source %q", id, srcID)
			}
		}
		for _, sinkID := range p.Sinks {
			if !knownSinks[sinkID] {
				return fmt.Errorf("pipeline %q: references unknown sink %q", id, sinkID)
			}
		}
	}

	return nil
}
