package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kafka             KafkaConfig    `yaml:"kafka"`
	Temporal          TemporalConfig `yaml:"temporal"`
	DB                DBConfig
	OperationDuration time.Duration `yaml:"operation_duration"`
}

type DBConfig struct {
	MasterDSN string `env:"MASTER_DSN"`
}

type TemporalConfig struct {
	HostPort                string `yaml:"host_port"`
	Namespace               string `yaml:"namespace"`
	TaskQueue               string `yaml:"task_queue"`
	MaxConcurrentActivities int    `yaml:"max_concurrent_activities"`
}

type KafkaConfig struct {
	Workers          int           `yaml:"workers"`
	TickTime         time.Duration `yaml:"tick_time"`
	Brokers          []string      `yaml:"brokers"`
	ConsumeTopic     string        `yaml:"consume_topic"`
	GroupID          string        `yaml:"group_id"`
	QueueCapacity    int           `yaml:"queue_capacity"`
	ReadBatchTimeout time.Duration `yaml:"read_batch_timeout"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		return nil, fmt.Errorf("error ProvideConfig: %w", err)
	}

	return cfg, nil
}
