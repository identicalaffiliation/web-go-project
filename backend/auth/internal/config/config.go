package config

import (
	"fmt"
	"os"
)

type Config struct {
	Postgres PostgresConfig
	JWT      JWTConfig
	HTTPAddr string
}

type PostgresConfig struct {
	DSN string
}

type JWTConfig struct {
	PrivateKeyPath string
	PublicKeyPath  string
	KeyID          string
	Issuer         string
}

func Load() (Config, error) {
	dsn, err := requireEnv("AUTH_DB_DSN")
	if err != nil {
		return Config{}, err
	}
	privPath, err := requireEnv("AUTH_JWT_PRIVATE_KEY_PATH")
	if err != nil {
		return Config{}, err
	}
	pubPath, err := requireEnv("AUTH_JWT_PUBLIC_KEY_PATH")
	if err != nil {
		return Config{}, err
	}
	keyID, err := requireEnv("AUTH_JWT_KEY_ID")
	if err != nil {
		return Config{}, err
	}
	issuer, err := requireEnv("AUTH_JWT_ISSUER")
	if err != nil {
		return Config{}, err
	}

	httpAddr := os.Getenv("AUTH_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	return Config{
		Postgres: PostgresConfig{DSN: dsn},
		JWT: JWTConfig{
			PrivateKeyPath: privPath,
			PublicKeyPath:  pubPath,
			KeyID:          keyID,
			Issuer:         issuer,
		},
		HTTPAddr: httpAddr,
	}, nil
}

func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("missing required environment variable %s", key)
	}
	return v, nil
}
