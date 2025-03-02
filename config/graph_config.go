package config

// Connector

type ConnectorConfig struct {
	ID ID `yaml:"id"`
}

type GraphConfig struct {
	Sources     map[ID]SourceConfig     `yaml:"sources"`
	StateStores map[ID]StateStoreConfig `yaml:"state_stores"`
	Connectors  map[ID]interface{}      `yaml:"connectors"`
	Sinks       map[ID]SinkConfig       `yaml:"sinks"`
	Pipelines   map[ID]PipelineConfig   `yaml:"pipelines"`
	Outlet      *SinkConfig             `yaml:"outlet"`
}
