package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
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
	DSN string `yaml:"-" env:"AUTH_DB_DSN" env-required:"true"`

	MaxConns          int32         `yaml:"max_conns" env:"AUTH_DB_MAX_CONNS"`
	MinConns          int32         `yaml:"min_conns" env:"AUTH_DB_MIN_CONNS"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime" env:"AUTH_DB_MAX_CONN_LIFETIME"`
	MaxConnIdleTime   time.Duration `yaml:"max_conn_idle_time" env:"AUTH_DB_MAX_CONN_IDLE_TIME"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period" env:"AUTH_DB_HEALTH_CHECK_PERIOD"`
	ConnectTimeout    time.Duration `yaml:"connect_timeout" env:"AUTH_DB_CONNECT_TIMEOUT"`
}

type JWTConfig struct {
	PrivateKeyPath string `yaml:"-" env:"AUTH_JWT_PRIVATE_KEY_PATH" env-required:"true"`
	PublicKeyPath  string `yaml:"-" env:"AUTH_JWT_PUBLIC_KEY_PATH" env-required:"true"`
	KeyID          string `yaml:"-" env:"AUTH_JWT_KEY_ID" env-required:"true"`

	Issuer     string        `yaml:"issuer" env:"AUTH_JWT_ISSUER"`
	AccessTTL  time.Duration `yaml:"access_ttl" env:"AUTH_ACCESS_TOKEN_TTL"`
	RefreshTTL time.Duration `yaml:"refresh_ttl" env:"AUTH_REFRESH_TOKEN_TTL"`
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
	return nil
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

func (c PostgresConfig) validate() error {
	// env-required ловит только отсутствующую переменную: пустая (AUTH_DB_DSN=)
	// для cleanenv считается заданной.
	if c.DSN == "" {
		return fmt.Errorf("%w: AUTH_DB_DSN must not be empty", ErrInvalid)
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
		{"AUTH_JWT_PRIVATE_KEY_PATH", c.PrivateKeyPath},
		{"AUTH_JWT_PUBLIC_KEY_PATH", c.PublicKeyPath},
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
