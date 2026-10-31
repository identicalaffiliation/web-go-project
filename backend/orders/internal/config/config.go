package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres DBConfig       `yaml:"postgres"`
	GRPC     GRPCServer     `yaml:"grpc"`
	HTTP     HTTPConfig     `yaml:"server"`
	Temporal TemporalConfig `yaml:"temporal"`
	Catalog  CatalogConfig  `yaml:"catalog"`
	// таймаут на подключение к базам при старте (создание пула + Ping)
	OperationDuration time.Duration `yaml:"operation_duration" env-default:"30s"`
}

type DBConfig struct {
	// yaml:"-": в DSN лежит пароль, из файла он не читается
	MasterDSN  string `yaml:"-" env:"MASTER_DSN"`
	ReplicaDSN string `yaml:"-" env:"REPLICA_DSN"`

	Master  PoolConfig `yaml:"master"`
	Replica PoolConfig `yaml:"replica"`
}

// PoolConfig - настройки pgxpool. Нулевое поле оставляет значение pgx по умолчанию.
type PoolConfig struct {
	// потолок соединений одного инстанса: суммарно по всем репликам сервиса
	// (и другим клиентам базы) не должен превышать max_connections postgres (100)
	MaxConns          int32         `yaml:"max_conns"`
	MinConns          int32         `yaml:"min_conns"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `yaml:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period"`
	ConnectTimeout    time.Duration `yaml:"connect_timeout"`
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
