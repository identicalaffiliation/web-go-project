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
	DBConfig          `yaml:"postgres"`
	HTTPConfig        HTTPConfig    `yaml:"server"`
	GRPCConfig        GRPCConfig    `yaml:"grpc"`
	FeedbacksGRPC     GRPCConfig    `yaml:"feedbacks_grpc"`
	MinioConfig       MinioConfig   `yaml:"s3"`
	OperationDuration time.Duration `yaml:"operation_duration"`
	Cache             RedisConfig   `yaml:"cache"`
	Broker            KafkaConfig   `yaml:"kafka"`
	Outbox            OutboxConfig  `yaml:"outbox"`
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

type HTTPConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type GRPCConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
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

// OutboxConfig - настройки outbox-воркеров, по одному на каждую outbox-таблицу.
type OutboxConfig struct {
	Stock OutboxWorkerConfig `yaml:"stock"`
	Likes OutboxWorkerConfig `yaml:"likes"`
}

type OutboxWorkerConfig struct {
	TickTime time.Duration `yaml:"tick"`
	Limit    int64         `yaml:"limit"`
	Topic    string        `yaml:"topic"`
}

type KafkaConfig struct {
	Brokers      []string      `yaml:"brokers"`
	BatchSize    int           `yaml:"batch_size"`
	BatchTimeout time.Duration `yaml:"batch_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}
