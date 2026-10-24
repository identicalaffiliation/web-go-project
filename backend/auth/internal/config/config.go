package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// ErrInvalid — значение прочитано, но не проходит проверку validate.
var ErrInvalid = errors.New("invalid config")

// У полей с тегом yaml значения по умолчанию живут в configs/auth.yml, а не в
// тегах env-default: с env-default cleanenv подменял бы явный ноль из файла
// (например, min_conns: 0), потому что не отличает его от «не задано».
type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Postgres PostgresConfig `yaml:"postgres"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Outbox   OutboxConfig   `yaml:"outbox"`

	RoleConsumer RoleConsumerConfig `yaml:"role_consumer"`
}

type HTTPConfig struct {
	Addr            string        `yaml:"addr" env:"AUTH_HTTP_ADDR"`
	ReadTimeout     time.Duration `yaml:"read_timeout" env:"AUTH_HTTP_READ_TIMEOUT"`
	WriteTimeout    time.Duration `yaml:"write_timeout" env:"AUTH_HTTP_WRITE_TIMEOUT"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" env:"AUTH_HTTP_IDLE_TIMEOUT"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"AUTH_HTTP_SHUTDOWN_TIMEOUT"`
}

type PostgresConfig struct {
	// yaml:"-": в DSN лежит пароль, поэтому из файла он не читается вообще.
	// Без тега yaml декодер взял бы ключ dsn по имени поля.
	DSN string `yaml:"-" env:"MASTER_DSN" env-required:"true"`

	MaxConns          int32         `yaml:"max_conns" env:"AUTH_DB_MAX_CONNS"`
	MinConns          int32         `yaml:"min_conns" env:"AUTH_DB_MIN_CONNS"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime" env:"AUTH_DB_MAX_CONN_LIFETIME"`
	MaxConnIdleTime   time.Duration `yaml:"max_conn_idle_time" env:"AUTH_DB_MAX_CONN_IDLE_TIME"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period" env:"AUTH_DB_HEALTH_CHECK_PERIOD"`
	ConnectTimeout    time.Duration `yaml:"connect_timeout" env:"AUTH_DB_CONNECT_TIMEOUT"`
}

type JWTConfig struct {
	PrivateKeyPath string `yaml:"-" env:"PRIVATE_KEY_PATH" env-required:"true"`
	PublicKeyPath  string `yaml:"-" env:"PUBLIC_KEY_PATH" env-required:"true"`
	KeyID          string `yaml:"-" env:"AUTH_JWT_KEY_ID" env-required:"true"`

	Issuer     string        `yaml:"issuer" env:"AUTH_JWT_ISSUER"`
	AccessTTL  time.Duration `yaml:"access_ttl" env:"AUTH_ACCESS_TOKEN_TTL"`
	RefreshTTL time.Duration `yaml:"refresh_ttl" env:"AUTH_REFRESH_TOKEN_TTL"`
}

// KafkaConfig: адреса брокеров не секрет, поэтому живут в файле; в окружении
// список задаётся через запятую (AUTH_KAFKA_BROKERS=k1:9092,k2:9092).
type KafkaConfig struct {
	Brokers      []string      `yaml:"brokers" env:"AUTH_KAFKA_BROKERS" env-separator:","`
	Topic        string        `yaml:"topic" env:"AUTH_KAFKA_TOPIC"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"AUTH_KAFKA_WRITE_TIMEOUT"`
}

type OutboxConfig struct {
	Interval       time.Duration `yaml:"interval" env:"AUTH_OUTBOX_INTERVAL"`
	BatchSize      int           `yaml:"batch_size" env:"AUTH_OUTBOX_BATCH_SIZE"`
	PublishTimeout time.Duration `yaml:"publish_timeout" env:"AUTH_OUTBOX_PUBLISH_TIMEOUT"`
	MaxAttempts    int           `yaml:"max_attempts" env:"AUTH_OUTBOX_MAX_ATTEMPTS"`
}

// RoleConsumerConfig — консьюмер события «создан бизнес-профиль». Адреса
// брокеров общие с kafka.brokers.
type RoleConsumerConfig struct {
	Topic   string `yaml:"topic" env:"AUTH_ROLE_CONSUMER_TOPIC"`
	GroupID string `yaml:"group_id" env:"AUTH_ROLE_CONSUMER_GROUP_ID"`
	// RetryBackoff — пауза перед повтором при сбое БД или чтения из Kafka.
	RetryBackoff  time.Duration `yaml:"retry_backoff" env:"AUTH_ROLE_CONSUMER_RETRY_BACKOFF"`
	CommitTimeout time.Duration `yaml:"commit_timeout" env:"AUTH_ROLE_CONSUMER_COMMIT_TIMEOUT"`
	MaxWait       time.Duration `yaml:"max_wait" env:"AUTH_ROLE_CONSUMER_MAX_WAIT"`
}

type LogConfig struct {
	// Строка, а не slog.Level: нулевое значение slog.Level — это info, и validate
	// не отличил бы опечатку в ключе log.level от явно заданного уровня.
	Level string `yaml:"level" env:"AUTH_LOG_LEVEL"`
}

// SlogLevel разбирает уровень логирования: debug, info, warn или error.
func (c LogConfig) SlogLevel() (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Level)); err != nil {
		return 0, fmt.Errorf("%w: log.level (AUTH_LOG_LEVEL): %w", ErrInvalid, err)
	}
	return level, nil
}

// Load читает файл по пути path, затем переменные окружения: окружение
// приоритетнее файла.
func Load(path string) (Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, fmt.Errorf("config file %q not found (set the path with -config): %w", path, err)
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if err := c.HTTP.validate(); err != nil {
		return err
	}
	if err := c.Postgres.validate(); err != nil {
		return err
	}
	if err := c.JWT.validate(); err != nil {
		return err
	}
	if _, err := c.Log.SlogLevel(); err != nil {
		return err
	}
	if err := c.Kafka.validate(); err != nil {
		return err
	}
	if err := c.Outbox.validate(); err != nil {
		return err
	}
	return c.RoleConsumer.validate()
}

func (c HTTPConfig) validate() error {
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return fmt.Errorf("%w: http.addr (AUTH_HTTP_ADDR) must be host:port, got %q", ErrInvalid, c.Addr)
	}
	return positiveDurations([]namedDuration{
		{"http.read_timeout (AUTH_HTTP_READ_TIMEOUT)", c.ReadTimeout},
		{"http.write_timeout (AUTH_HTTP_WRITE_TIMEOUT)", c.WriteTimeout},
		{"http.idle_timeout (AUTH_HTTP_IDLE_TIMEOUT)", c.IdleTimeout},
		{"http.shutdown_timeout (AUTH_HTTP_SHUTDOWN_TIMEOUT)", c.ShutdownTimeout},
	})
}

func (c KafkaConfig) validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("%w: kafka.brokers (AUTH_KAFKA_BROKERS) must not be empty", ErrInvalid)
	}
	// Пустая переменная AUTH_KAFKA_BROKERS= даёт список из одной пустой строки.
	for _, b := range c.Brokers {
		if strings.TrimSpace(b) == "" {
			return fmt.Errorf("%w: kafka.brokers (AUTH_KAFKA_BROKERS) contains an empty address", ErrInvalid)
		}
	}
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("%w: kafka.topic (AUTH_KAFKA_TOPIC) must not be empty", ErrInvalid)
	}
	return positiveDurations([]namedDuration{
		{"kafka.write_timeout (AUTH_KAFKA_WRITE_TIMEOUT)", c.WriteTimeout},
	})
}

func (c OutboxConfig) validate() error {
	if c.BatchSize < 1 {
		return fmt.Errorf("%w: outbox.batch_size (AUTH_OUTBOX_BATCH_SIZE) must be >= 1, got %d", ErrInvalid, c.BatchSize)
	}
	if c.MaxAttempts < 1 {
		return fmt.Errorf("%w: outbox.max_attempts (AUTH_OUTBOX_MAX_ATTEMPTS) must be >= 1, got %d", ErrInvalid, c.MaxAttempts)
	}
	return positiveDurations([]namedDuration{
		{"outbox.interval (AUTH_OUTBOX_INTERVAL)", c.Interval},
		{"outbox.publish_timeout (AUTH_OUTBOX_PUBLISH_TIMEOUT)", c.PublishTimeout},
	})
}

func (c RoleConsumerConfig) validate() error {
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("%w: role_consumer.topic (AUTH_ROLE_CONSUMER_TOPIC) must not be empty", ErrInvalid)
	}
	if strings.TrimSpace(c.GroupID) == "" {
		return fmt.Errorf("%w: role_consumer.group_id (AUTH_ROLE_CONSUMER_GROUP_ID) must not be empty", ErrInvalid)
	}
	return positiveDurations([]namedDuration{
		{"role_consumer.retry_backoff (AUTH_ROLE_CONSUMER_RETRY_BACKOFF)", c.RetryBackoff},
		{"role_consumer.commit_timeout (AUTH_ROLE_CONSUMER_COMMIT_TIMEOUT)", c.CommitTimeout},
		{"role_consumer.max_wait (AUTH_ROLE_CONSUMER_MAX_WAIT)", c.MaxWait},
	})
}

func (c PostgresConfig) validate() error {
	// env-required ловит только отсутствующую переменную: пустая (MASTER_DSN=)
	// для cleanenv считается заданной.
	if c.DSN == "" {
		return fmt.Errorf("%w: MASTER_DSN must not be empty", ErrInvalid)
	}
	if c.MaxConns < 1 {
		return fmt.Errorf("%w: postgres.max_conns (AUTH_DB_MAX_CONNS) must be >= 1, got %d", ErrInvalid, c.MaxConns)
	}
	if c.MinConns < 0 {
		return fmt.Errorf("%w: postgres.min_conns (AUTH_DB_MIN_CONNS) must be >= 0, got %d", ErrInvalid, c.MinConns)
	}
	if c.MinConns > c.MaxConns {
		return fmt.Errorf("%w: postgres.min_conns (%d) must not exceed postgres.max_conns (%d)",
			ErrInvalid, c.MinConns, c.MaxConns)
	}
	return positiveDurations([]namedDuration{
		{"postgres.max_conn_lifetime (AUTH_DB_MAX_CONN_LIFETIME)", c.MaxConnLifetime},
		{"postgres.max_conn_idle_time (AUTH_DB_MAX_CONN_IDLE_TIME)", c.MaxConnIdleTime},
		{"postgres.health_check_period (AUTH_DB_HEALTH_CHECK_PERIOD)", c.HealthCheckPeriod},
		{"postgres.connect_timeout (AUTH_DB_CONNECT_TIMEOUT)", c.ConnectTimeout},
	})
}

func (c JWTConfig) validate() error {
	for _, s := range []struct{ name, value string }{
		{"PRIVATE_KEY_PATH", c.PrivateKeyPath},
		{"PUBLIC_KEY_PATH", c.PublicKeyPath},
		{"AUTH_JWT_KEY_ID", c.KeyID},
		{"jwt.issuer (AUTH_JWT_ISSUER)", c.Issuer},
	} {
		if s.value == "" {
			return fmt.Errorf("%w: %s must not be empty", ErrInvalid, s.name)
		}
	}
	return positiveDurations([]namedDuration{
		{"jwt.access_ttl (AUTH_ACCESS_TOKEN_TTL)", c.AccessTTL},
		{"jwt.refresh_ttl (AUTH_REFRESH_TOKEN_TTL)", c.RefreshTTL},
	})
}

type namedDuration struct {
	name  string
	value time.Duration
}

func positiveDurations(durations []namedDuration) error {
	for _, d := range durations {
		if d.value <= 0 {
			return fmt.Errorf("%w: %s must be > 0, got %s", ErrInvalid, d.name, d.value)
		}
	}
	return nil
}
