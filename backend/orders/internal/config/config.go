package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres DBConfig
	GRPC     GRPCServer `yaml:"grpc"`
}

type DBConfig struct {
	MasterDSN  string `env:"MASTER_DSN"`
	ReplicaDSN string `env:"REPLICA_DSN"`
}

type GRPCServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		return nil, fmt.Errorf("error ProvideConfig: %w", err)
	}

	return cfg, nil
}
