package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kafka KafkaConfig `yaml:"kafka"`
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
