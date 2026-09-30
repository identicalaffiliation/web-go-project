package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

func LoadConfig() (*Config, error) {
	cfg := new(Config)
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read envs: %w", err)
	}

	return cfg, nil
}
