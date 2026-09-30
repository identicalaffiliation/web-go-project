package config

type Config struct {
	MigrationsPath string `env:"MIGRATIONS_PATH"`
	DbUrl          string `env:"DB_URL"`
}
