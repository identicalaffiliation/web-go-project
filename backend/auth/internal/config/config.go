package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres PostgresConfig
	JWT      JWTConfig
	Kafka    KafkaConfig
	Outbox   OutboxConfig
	HTTPAddr string `env:"AUTH_HTTP_ADDR" env-default:":8080"`

	HTTPReadTimeout  time.Duration `env:"AUTH_HTTP_READ_TIMEOUT" env-default:"5s"`
	HTTPWriteTimeout time.Duration `env:"AUTH_HTTP_WRITE_TIMEOUT" env-default:"10s"`
	HTTPIdleTimeout  time.Duration `env:"AUTH_HTTP_IDLE_TIMEOUT" env-default:"60s"`

	LogLevel slog.Level `env:"AUTH_LOG_LEVEL" env-default:"info"`
}

type PostgresConfig struct {
	DSN string `env:"AUTH_DB_DSN" env-required:"true"`

	MaxConns          int32         `env:"AUTH_DB_MAX_CONNS" env-default:"20"`
	MinConns          int32         `env:"AUTH_DB_MIN_CONNS" env-default:"2"`
	MaxConnLifetime   time.Duration `env:"AUTH_DB_MAX_CONN_LIFETIME" env-default:"30m"`
	MaxConnIdleTime   time.Duration `env:"AUTH_DB_MAX_CONN_IDLE_TIME" env-default:"5m"`
	HealthCheckPeriod time.Duration `env:"AUTH_DB_HEALTH_CHECK_PERIOD" env-default:"30s"`
	ConnectTimeout    time.Duration `env:"AUTH_DB_CONNECT_TIMEOUT" env-default:"5s"`
}

type JWTConfig struct {
	PrivateKeyPath string        `env:"AUTH_JWT_PRIVATE_KEY_PATH" env-required:"true"`
	PublicKeyPath  string        `env:"AUTH_JWT_PUBLIC_KEY_PATH" env-required:"true"`
	KeyID          string        `env:"AUTH_JWT_KEY_ID" env-required:"true"`
	Issuer         string        `env:"AUTH_JWT_ISSUER" env-required:"true"`
	AccessTTL      time.Duration `env:"AUTH_ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshTTL     time.Duration `env:"AUTH_REFRESH_TOKEN_TTL" env-default:"168h"`
}

type KafkaConfig struct {
	Brokers      []string      `env:"AUTH_KAFKA_BROKERS" env-separator:"," env-required:"true"`
	Topic        string        `env:"AUTH_KAFKA_TOPIC" env-default:"users.created"`
	WriteTimeout time.Duration `env:"AUTH_KAFKA_WRITE_TIMEOUT" env-default:"5s"`
}

type OutboxConfig struct {
	Interval       time.Duration `env:"AUTH_OUTBOX_INTERVAL" env-default:"1s"`
	BatchSize      int           `env:"AUTH_OUTBOX_BATCH_SIZE" env-default:"100"`
	PublishTimeout time.Duration `env:"AUTH_OUTBOX_PUBLISH_TIMEOUT" env-default:"10s"`
	MaxAttempts    int           `env:"AUTH_OUTBOX_MAX_ATTEMPTS" env-default:"5"`
}

func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read config from env: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.JWT.AccessTTL <= 0 {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_TTL must be positive, got %s", c.JWT.AccessTTL)
	}
	if c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_TOKEN_TTL must be positive, got %s", c.JWT.RefreshTTL)
	}
	
	for _, d := range []struct {
		name  string
		value time.Duration
	}{
		{"AUTH_HTTP_READ_TIMEOUT", c.HTTPReadTimeout},
		{"AUTH_HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout},
		{"AUTH_HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout},
	} {
		if d.value <= 0 {
			return fmt.Errorf("%s must be > 0, got %v", d.name, d.value)
		}
	}
	if err := c.Kafka.validate(); err != nil {
		return err
	}
	if err := c.Outbox.validate(); err != nil {
		return err
	}
	return c.Postgres.validate()
}

func (c KafkaConfig) validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("AUTH_KAFKA_BROKERS must not be empty")
	}
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("AUTH_KAFKA_TOPIC must not be empty")
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("AUTH_KAFKA_WRITE_TIMEOUT must be > 0, got %v", c.WriteTimeout)
	}
	return nil
}

func (c OutboxConfig) validate() error {
	if c.Interval <= 0 {
		return fmt.Errorf("AUTH_OUTBOX_INTERVAL must be > 0, got %v", c.Interval)
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("AUTH_OUTBOX_BATCH_SIZE must be >= 1, got %d", c.BatchSize)
	}
	if c.MaxAttempts < 1 {
		return fmt.Errorf("AUTH_OUTBOX_MAX_ATTEMPTS must be >= 1, got %d", c.MaxAttempts)
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("AUTH_OUTBOX_PUBLISH_TIMEOUT must be > 0, got %v", c.PublishTimeout)
	}
	return nil
}

func (c PostgresConfig) validate() error {
	if c.MaxConns < 1 {
		return fmt.Errorf("AUTH_DB_MAX_CONNS must be >= 1, got %d", c.MaxConns)
	}
	if c.MinConns < 0 {
		return fmt.Errorf("AUTH_DB_MIN_CONNS must be >= 0, got %d", c.MinConns)
	}
	if c.MinConns > c.MaxConns {
		return fmt.Errorf("AUTH_DB_MIN_CONNS (%d) must not exceed AUTH_DB_MAX_CONNS (%d)",
			c.MinConns, c.MaxConns)
	}

	durations := []struct {
		name  string
		value time.Duration
	}{
		{"AUTH_DB_MAX_CONN_LIFETIME", c.MaxConnLifetime},
		{"AUTH_DB_MAX_CONN_IDLE_TIME", c.MaxConnIdleTime},
		{"AUTH_DB_HEALTH_CHECK_PERIOD", c.HealthCheckPeriod},
		{"AUTH_DB_CONNECT_TIMEOUT", c.ConnectTimeout},
	}
	for _, d := range durations {
		if d.value <= 0 {
			return fmt.Errorf("%s must be > 0, got %v", d.name, d.value)
		}
	}

	return nil
}
