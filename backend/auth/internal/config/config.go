package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres PostgresConfig
	JWT      JWTConfig
	HTTPAddr string `env:"AUTH_HTTP_ADDR" env-default:":8080"`
	
	LogLevel slog.Level `env:"AUTH_LOG_LEVEL" env-default:"info"`
}

type PostgresConfig struct {
	DSN string `env:"AUTH_DB_DSN" env-required:"true"`
}

type JWTConfig struct {
	PrivateKeyPath string        `env:"AUTH_JWT_PRIVATE_KEY_PATH" env-required:"true"`
	PublicKeyPath  string        `env:"AUTH_JWT_PUBLIC_KEY_PATH" env-required:"true"`
	KeyID          string        `env:"AUTH_JWT_KEY_ID" env-required:"true"`
	Issuer         string        `env:"AUTH_JWT_ISSUER" env-required:"true"`
	AccessTTL      time.Duration `env:"AUTH_ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshTTL     time.Duration `env:"AUTH_REFRESH_TOKEN_TTL" env-default:"168h"`
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

// validate проверяет то, что теги cleanenv выразить не могут: нулевой или
// отрицательный TTL превратил бы выданные токены в мгновенно протухшие.
func (c Config) validate() error {
	if c.JWT.AccessTTL <= 0 {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_TTL must be positive, got %s", c.JWT.AccessTTL)
	}
	if c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_TOKEN_TTL must be positive, got %s", c.JWT.RefreshTTL)
	}
	return nil
}
