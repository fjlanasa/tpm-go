package config

// Source

type SourceType string

const (
	SourceTypeHTTP       SourceType = "http"
	SourceTypeBucket     SourceType = "bucket"
	SourceTypeFileSystem SourceType = "file_system"
	SourceTypeRedis      SourceType = "redis"
	SourceTypeConnector  SourceType = "connector"
)

type HTTPSourceConfig struct {
	URL      string `yaml:"url"`
	Interval string `yaml:"interval"`
}

type BucketSourceConfig struct {
	URL        string `yaml:"url"`
	BucketName string `yaml:"bucket_name"`
}

type FileSystemSourceConfig struct {
	Path string `yaml:"path"`
}

type RedisSourceConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Channel  string `yaml:"channel"`
}

type SourceConfig struct {
	ID   ID               `yaml:"id"`
	Type SourceType       `yaml:"type"`
	HTTP HTTPSourceConfig `yaml:"http"`
	// Destinations
	// Object Storage
	Bucket BucketSourceConfig `yaml:"bucket"`
	// File System
	FileSystem FileSystemSourceConfig `yaml:"file_system"`
	// Redis
	Redis RedisSourceConfig `yaml:"redis"`
	// Connector Channel
	Connector ConnectorConfig `yaml:"connector"`
}
