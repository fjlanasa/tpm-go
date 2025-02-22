package config

import "time"

// StateStore

type StateStoreType string

const (
	InMemoryStateStoreType StateStoreType = "in_memory"
	RedisStateStoreType    StateStoreType = "redis"
)

type InMemoryStateStoreConfig struct {
	Expiry time.Duration `yaml:"expiry"`
}

type RedisStateStoreConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	Expiry   time.Duration `yaml:"expiry"`
}

type StateStoreConfig struct {
	ID       ID                       `yaml:"id"`
	Type     StateStoreType           `yaml:"type"`
	InMemory InMemoryStateStoreConfig `yaml:"in_memory"`
	Redis    RedisStateStoreConfig    `yaml:"redis"`
}
