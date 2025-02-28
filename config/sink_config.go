package config

import "time"

type SinkType string

const (
	SinkTypeConnector  SinkType = "connector"
	SinkTypeConsole    SinkType = "console"
	SinkTypeFileSystem SinkType = "file_system"
	SinkTypeRedis      SinkType = "redis"
)

type ConsoleSinkLevel string

const (
	ConsoleSinkLevelDebug   ConsoleSinkLevel = "debug"
	ConsoleSinkLevelInfo    ConsoleSinkLevel = "info"
	ConsoleSinkLevelWarning ConsoleSinkLevel = "warning"
	ConsoleSinkLevelError   ConsoleSinkLevel = "error"
	ConsoleSinkLevelFatal   ConsoleSinkLevel = "fatal"
)

type BucketSinkConfig struct {
	URL        string `yaml:"url"`
	BucketName string `yaml:"bucket_name"`
}

type ConsoleSinkConfig struct {
	Level ConsoleSinkLevel `yaml:"level"`
}

type FileSystemSinkConfig struct {
	Path string `yaml:"path"`
}

type RedisSinkConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Channel  string `yaml:"channel"`
}

type SinkConfig struct {
	ID             ID            `yaml:"id"`
	Type           SinkType      `yaml:"type"`
	MinBatchSize   int           `yaml:"min_batch_size"`
	MaxBatchSize   int           `yaml:"max_batch_size"`
	FlushInterval  time.Duration `yaml:"flush_interval"`
	FlushThreshold int           `yaml:"flush_threshold"`
	// Destinations
	// Console
	Console ConsoleSinkConfig `yaml:"console"`
	// Object Storage
	Bucket BucketSinkConfig `yaml:"bucket"`
	// File System
	FileSystem FileSystemSinkConfig `yaml:"file_system"`
	// Redis
	Redis RedisSinkConfig `yaml:"redis"`
	// Connector Channel
	Connector ConnectorConfig `yaml:"connector"`
}
