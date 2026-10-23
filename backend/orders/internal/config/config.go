package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres DBConfig
	GRPC     GRPCServer     `yaml:"grpc"`
	HTTP     HTTPConfig     `yaml:"server"`
	Temporal TemporalConfig `yaml:"temporal"`
	Catalog  CatalogConfig  `yaml:"catalog"`
}

type DBConfig struct {
	MasterDSN  string `env:"MASTER_DSN"`
	ReplicaDSN string `env:"REPLICA_DSN"`
}

type TemporalConfig struct {
	HostPort                string `yaml:"host_port"`
	Namespace               string `yaml:"namespace"`
	TaskQueue               string `yaml:"task_queue"`
	MaxConcurrentActivities int    `yaml:"max_concurrent_activities"`
}

type CatalogConfig struct {
	Addr string `yaml:"addr"`
}

type GRPCServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type HTTPConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		return nil, fmt.Errorf("error ProvideConfig: %w", err)
	}

	return cfg, nil
}
