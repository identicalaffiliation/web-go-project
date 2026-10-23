package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres          DBConfig
	GRPC              GRPCServer     `yaml:"grpc"`
	Kafka             KafkaConfig    `yaml:"kafka"`
	Temporal          TemporalConfig `yaml:"temporal"`
	InitialBalance    int64          `yaml:"initial_balance"`
	OperationDuration time.Duration  `yaml:"operation_duration"`
}

type DBConfig struct {
	MasterDSN  string `env:"MASTER_DSN"`
	ReplicaDSN string `env:"REPLICA_DSN"`
}

type GRPCServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type KafkaConfig struct {
	Workers          int           `yaml:"workers"`
	Brokers          []string      `yaml:"brokers"`
	ConsumeTopic     string        `yaml:"consume_topic"`
	GroupID          string        `yaml:"group_id"`
	QueueCapacity    int           `yaml:"queue_capacity"`
	ReadBatchTimeout time.Duration `yaml:"read_batch_timeout"`
	RetryBackoff     time.Duration `yaml:"retry_backoff"`
}

type TemporalConfig struct {
	HostPort                string `yaml:"host_port"`
	Namespace               string `yaml:"namespace"`
	TaskQueue               string `yaml:"task_queue"`
	MaxConcurrentActivities int    `yaml:"max_concurrent_activities"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		return nil, fmt.Errorf("error ProvideConfig: %w", err)
	}

	return cfg, nil
}
