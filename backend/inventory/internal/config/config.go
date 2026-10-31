package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kafka             KafkaConfig    `yaml:"kafka"`
	Temporal          TemporalConfig `yaml:"temporal"`
	DB                DBConfig       `yaml:"postgres"`
	OperationDuration time.Duration  `yaml:"operation_duration"`
}

type DBConfig struct {
	// yaml:"-": в DSN лежит пароль, из файла он не читается
	MasterDSN string `yaml:"-" env:"MASTER_DSN"`

	Master PoolConfig `yaml:"master"`
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
