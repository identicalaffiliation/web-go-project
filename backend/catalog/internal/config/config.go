package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

func ProvideConfig(configPath string) (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		return nil, fmt.Errorf("error ProvideConfig: %w", err)
	}

	return cfg, nil
}

type Config struct {
	DBConfig
	HTTPConfig        HTTPConfig    `yaml:"server"`
	MinioConfig       MinioConfig   `yaml:"s3"`
	OperationDuration time.Duration `yaml:"operation_duration"`
	Cache             RedisConfig   `yaml:"cache"`
}

type DBConfig struct {
	MasterDSN  string `env:"MASTER_DSN"`
	ReplicaDSN string `env:"REPLICA_DSN"`
}

type HTTPConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type MinioConfig struct {
	InternalHost string        `yaml:"internal_host"`
	ExternalHost string        `yaml:"external_host"`
	Port         int           `yaml:"port"`
	Bucket       string        `yaml:"bucket"`
	Expiration   time.Duration `yaml:"expiration"`
	User         string        `env:"MINIO_USER"`
	Password     string        `env:"MINIO_PASSWORD"`
}

type RedisConfig struct {
	Host     string        `yaml:"host"`
	Port     int           `yaml:"port"`
	Password string        `env:"REDIS_PASSWORD"`
	TTL      time.Duration `yaml:"ttl"`
}
