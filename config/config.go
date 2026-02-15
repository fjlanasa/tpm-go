package config

import (
	"os"

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
	return &Config{
		Graph: config.Graph,
	}, nil
}
