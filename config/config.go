package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ID string

type ConfigYaml struct {
	EventServer *EventServerConfig `yaml:"event_server"`
	Graph       *GraphConfig       `yaml:"graph"`
}

type Config struct {
	EventServer *EventServerConfig
	Graph       *GraphConfig
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
	if err != nil {
		return nil, err
	}
	return &Config{
		EventServer: config.EventServer,
		Graph:       config.Graph,
	}, nil
}
