package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const realConfigPath = "../../configs/auth.yml"

var allEnvKeys = []string{
	"MASTER_DSN",
	"PRIVATE_KEY_PATH",
	"PUBLIC_KEY_PATH",
	"AUTH_JWT_KEY_ID",
	"AUTH_JWT_ISSUER",
	"AUTH_HTTP_ADDR",
	"AUTH_ACCESS_TOKEN_TTL",
	"AUTH_REFRESH_TOKEN_TTL",
	"AUTH_LOG_LEVEL",
	"AUTH_HTTP_READ_TIMEOUT",
	"AUTH_HTTP_WRITE_TIMEOUT",
	"AUTH_HTTP_IDLE_TIMEOUT",
	"AUTH_HTTP_SHUTDOWN_TIMEOUT",
	"AUTH_DB_MAX_CONNS",
	"AUTH_DB_MIN_CONNS",
	"AUTH_DB_MAX_CONN_LIFETIME",
	"AUTH_DB_MAX_CONN_IDLE_TIME",
	"AUTH_DB_HEALTH_CHECK_PERIOD",
	"AUTH_DB_CONNECT_TIMEOUT",
	"AUTH_KAFKA_BROKERS",
	"AUTH_KAFKA_TOPIC",
	"AUTH_KAFKA_WRITE_TIMEOUT",
	"AUTH_OUTBOX_INTERVAL",
	"AUTH_OUTBOX_BATCH_SIZE",
	"AUTH_OUTBOX_PUBLISH_TIMEOUT",
	"AUTH_OUTBOX_MAX_ATTEMPTS",
	"AUTH_ROLE_CONSUMER_TOPIC",
	"AUTH_ROLE_CONSUMER_GROUP_ID",
	"AUTH_ROLE_CONSUMER_RETRY_BACKOFF",
	"AUTH_ROLE_CONSUMER_COMMIT_TIMEOUT",
	"AUTH_ROLE_CONSUMER_MAX_WAIT",
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unset %s: %v", k, err)
		}
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func requiredVars() map[string]string {
	return map[string]string{
		"MASTER_DSN":       "postgres://u:p@localhost:5432/auth",
		"PRIVATE_KEY_PATH": "/keys/private.pem",
		"PUBLIC_KEY_PATH":  "/keys/public.pem",
		"AUTH_JWT_KEY_ID":  "kid-1",
	}
}

func withOverrides(overrides map[string]string) map[string]string {
	vars := requiredVars()
	for k, v := range overrides {
		vars[k] = v
	}
	return vars
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// fileDefaults — то, что должно получиться из configs/auth.yml и requiredVars.
func fileDefaults() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:            ":8080",
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Postgres: PostgresConfig{
			DSN:               "postgres://u:p@localhost:5432/auth",
			MaxConns:          20,
			MinConns:          2,
			MaxConnLifetime:   30 * time.Minute,
			MaxConnIdleTime:   5 * time.Minute,
			HealthCheckPeriod: 30 * time.Second,
			ConnectTimeout:    5 * time.Second,
		},
		JWT: JWTConfig{
			PrivateKeyPath: "/keys/private.pem",
			PublicKeyPath:  "/keys/public.pem",
			KeyID:          "kid-1",
			Issuer:         "auth-service",
			AccessTTL:      15 * time.Minute,
			RefreshTTL:     7 * 24 * time.Hour,
		},
		Log: LogConfig{Level: "info"},
		Kafka: KafkaConfig{
			Brokers:      []string{"shop-kafka:9092"},
			Topic:        "users.created",
			WriteTimeout: 5 * time.Second,
		},
		Outbox: OutboxConfig{Interval: time.Second, BatchSize: 100, PublishTimeout: 10 * time.Second, MaxAttempts: 5},
		RoleConsumer: RoleConsumerConfig{
			Topic:         "profiles.business.created",
			GroupID:       "auth-service",
			RetryBackoff:  2 * time.Second,
			CommitTimeout: 5 * time.Second,
			MaxWait:       500 * time.Millisecond,
		},
	}
}

// Читает настоящий configs/auth.yml: опечатка в ключе оставит поле нулевым,
// и тест упадёт на validate или на сравнении.
func TestLoad_RealFile(t *testing.T) {
	setEnv(t, requiredVars())

	cfg, err := Load(realConfigPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// DeepEqual, а не ==: в Kafka.Brokers лежит срез, и Config перестал быть comparable.
	if want := fileDefaults(); !reflect.DeepEqual(cfg, want) {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestLoad_KafkaBrokersList(t *testing.T) {
	setEnv(t, withOverrides(map[string]string{"AUTH_KAFKA_BROKERS": "k1:9092,k2:9092"}))

	cfg, err := Load(realConfigPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"k1:9092", "k2:9092"}; !reflect.DeepEqual(cfg.Kafka.Brokers, want) {
		t.Errorf("Brokers = %v, want %v", cfg.Kafka.Brokers, want)
	}
}

func TestLoad_InvalidKafkaAndOutbox(t *testing.T) {
	tests := map[string]map[string]string{
		"empty topic":          {"AUTH_KAFKA_TOPIC": " "},
		"empty broker in list": {"AUTH_KAFKA_BROKERS": "k1:9092,,k2:9092"},
		"zero write timeout":   {"AUTH_KAFKA_WRITE_TIMEOUT": "0s"},
		"zero outbox interval": {"AUTH_OUTBOX_INTERVAL": "0s"},
		"zero batch size":      {"AUTH_OUTBOX_BATCH_SIZE": "0"},
		"negative publish":     {"AUTH_OUTBOX_PUBLISH_TIMEOUT": "-1s"},
		"zero max attempts":    {"AUTH_OUTBOX_MAX_ATTEMPTS": "0"},

		"empty role topic":        {"AUTH_ROLE_CONSUMER_TOPIC": " "},
		"empty role group":        {"AUTH_ROLE_CONSUMER_GROUP_ID": " "},
		"zero role retry backoff": {"AUTH_ROLE_CONSUMER_RETRY_BACKOFF": "0s"},
		"negative role commit":    {"AUTH_ROLE_CONSUMER_COMMIT_TIMEOUT": "-1s"},
		"zero role max wait":      {"AUTH_ROLE_CONSUMER_MAX_WAIT": "0s"},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			setEnv(t, withOverrides(overrides))

			if _, err := Load(realConfigPath); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid for %v", err, overrides)
			}
		})
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	setEnv(t, withOverrides(map[string]string{
		"AUTH_HTTP_ADDR":              ":9090",
		"AUTH_HTTP_READ_TIMEOUT":      "2s",
		"AUTH_HTTP_WRITE_TIMEOUT":     "4s",
		"AUTH_HTTP_IDLE_TIMEOUT":      "30s",
		"AUTH_HTTP_SHUTDOWN_TIMEOUT":  "20s",
		"AUTH_DB_MAX_CONNS":           "10",
		"AUTH_DB_MIN_CONNS":           "10",
		"AUTH_DB_MAX_CONN_LIFETIME":   "1h",
		"AUTH_DB_MAX_CONN_IDLE_TIME":  "10m",
		"AUTH_DB_HEALTH_CHECK_PERIOD": "1m",
		"AUTH_DB_CONNECT_TIMEOUT":     "3s",
		"AUTH_JWT_ISSUER":             "other-issuer",
		"AUTH_ACCESS_TOKEN_TTL":       "5m",
		"AUTH_REFRESH_TOKEN_TTL":      "24h",
		"AUTH_LOG_LEVEL":              "debug",
		"AUTH_KAFKA_BROKERS":          "k1:9092,k2:9092",
		"AUTH_KAFKA_TOPIC":            "other.topic",
		"AUTH_KAFKA_WRITE_TIMEOUT":    "2s",
		"AUTH_OUTBOX_INTERVAL":        "3s",
		"AUTH_OUTBOX_BATCH_SIZE":      "50",
		"AUTH_OUTBOX_PUBLISH_TIMEOUT": "4s",
		"AUTH_OUTBOX_MAX_ATTEMPTS":    "7",

		"AUTH_ROLE_CONSUMER_TOPIC":          "other.business",
		"AUTH_ROLE_CONSUMER_GROUP_ID":       "other-group",
		"AUTH_ROLE_CONSUMER_RETRY_BACKOFF":  "9s",
		"AUTH_ROLE_CONSUMER_COMMIT_TIMEOUT": "8s",
		"AUTH_ROLE_CONSUMER_MAX_WAIT":       "1s",
	}))

	cfg, err := Load(realConfigPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Config{
		HTTP: HTTPConfig{
			Addr:            ":9090",
			ReadTimeout:     2 * time.Second,
			WriteTimeout:    4 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 20 * time.Second,
		},
		Postgres: PostgresConfig{
			DSN:               "postgres://u:p@localhost:5432/auth",
			MaxConns:          10,
			MinConns:          10,
			MaxConnLifetime:   time.Hour,
			MaxConnIdleTime:   10 * time.Minute,
			HealthCheckPeriod: time.Minute,
			ConnectTimeout:    3 * time.Second,
		},
		JWT: JWTConfig{
			PrivateKeyPath: "/keys/private.pem",
			PublicKeyPath:  "/keys/public.pem",
			KeyID:          "kid-1",
			Issuer:         "other-issuer",
			AccessTTL:      5 * time.Minute,
			RefreshTTL:     24 * time.Hour,
		},
		Log: LogConfig{Level: "debug"},
		Kafka: KafkaConfig{
			Brokers:      []string{"k1:9092", "k2:9092"},
			Topic:        "other.topic",
			WriteTimeout: 2 * time.Second,
		},
		Outbox: OutboxConfig{Interval: 3 * time.Second, BatchSize: 50, PublishTimeout: 4 * time.Second, MaxAttempts: 7},
		RoleConsumer: RoleConsumerConfig{
			Topic:         "other.business",
			GroupID:       "other-group",
			RetryBackoff:  9 * time.Second,
			CommitTimeout: 8 * time.Second,
			MaxWait:       time.Second,
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
}

// Одна переменная меняет одно поле, остальные остаются из файла.
func TestLoad_SingleEnvOverride(t *testing.T) {
	setEnv(t, withOverrides(map[string]string{"AUTH_HTTP_SHUTDOWN_TIMEOUT": "45s"}))

	cfg, err := Load(realConfigPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := fileDefaults()
	want.HTTP.ShutdownTimeout = 45 * time.Second
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
}

// Явный ноль из файла должен сохраниться: ради этого убраны теги env-default.
func TestLoad_ZeroMinConnsFromFile(t *testing.T) {
	content, err := os.ReadFile(realConfigPath)
	if err != nil {
		t.Fatalf("read real config: %v", err)
	}
	path := writeConfig(t, strings.Replace(string(content), "min_conns: 2", "min_conns: 0", 1))
	setEnv(t, requiredVars())

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Postgres.MinConns != 0 {
		t.Errorf("MinConns = %d, want 0", cfg.Postgres.MinConns)
	}
}

// Пустая переменная для cleanenv считается заданной и затирает значение из
// файла, поэтому она должна давать ошибку, а не тихо обнулять настройку.
func TestLoad_EmptyEnvVar(t *testing.T) {
	for _, key := range allEnvKeys {
		t.Run(key, func(t *testing.T) {
			setEnv(t, withOverrides(map[string]string{key: ""}))

			if _, err := Load(realConfigPath); err == nil {
				t.Fatalf("expected error for empty %s", key)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	setEnv(t, requiredVars())
	path := filepath.Join(t.TempDir(), "missing.yml")

	_, err := Load(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "missing.yml") {
		t.Errorf("error %q does not mention the config path", err)
	}
}

func TestLoad_MissingRequiredEnv(t *testing.T) {
	for missing := range requiredVars() {
		t.Run(missing, func(t *testing.T) {
			vars := requiredVars()
			delete(vars, missing)
			setEnv(t, vars)

			if _, err := Load(realConfigPath); err == nil {
				t.Fatalf("expected error when %s is not set", missing)
			}
		})
	}
}

// Секреты из файла не читаются, даже если кто-то их туда положил.
func TestLoad_SecretsIgnoredInFile(t *testing.T) {
	content, err := os.ReadFile(realConfigPath)
	if err != nil {
		t.Fatalf("read real config: %v", err)
	}
	path := writeConfig(t, strings.Replace(string(content),
		"postgres:\n", "postgres:\n  dsn: postgres://u:p@localhost:5432/auth\n", 1))

	vars := requiredVars()
	delete(vars, "MASTER_DSN")
	setEnv(t, vars)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error: DSN must come from MASTER_DSN only")
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	tests := map[string]string{
		"empty file":     "",
		"malformed yaml": "http: [unclosed",
		"wrong type":     "http:\n  read_timeout: fast\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			setEnv(t, requiredVars())

			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// Опечатку в ключе YAML-декодер молча пропускает; ловит её validate.
func TestLoad_MisspelledKey(t *testing.T) {
	content, err := os.ReadFile(realConfigPath)
	if err != nil {
		t.Fatalf("read real config: %v", err)
	}

	for _, key := range []string{
		"addr", "read_timeout", "write_timeout", "idle_timeout", "shutdown_timeout",
		"max_conns", "max_conn_lifetime", "max_conn_idle_time", "health_check_period",
		"connect_timeout", "issuer", "access_ttl", "refresh_ttl", "level",
		"brokers", "topic", "interval", "batch_size", "publish_timeout", "max_attempts",
		"group_id", "retry_backoff", "commit_timeout", "max_wait",
	} {
		t.Run(key, func(t *testing.T) {
			old := "  " + key + ":"
			if !strings.Contains(string(content), old) {
				t.Fatalf("key %q not found in %s", key, realConfigPath)
			}
			path := writeConfig(t, strings.Replace(string(content), old, "  "+key+"_typo:", 1))
			setEnv(t, requiredVars())

			if _, err := Load(path); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	tests := map[string]map[string]string{
		"addr without port":         {"AUTH_HTTP_ADDR": "localhost"},
		"zero read timeout":         {"AUTH_HTTP_READ_TIMEOUT": "0s"},
		"negative write timeout":    {"AUTH_HTTP_WRITE_TIMEOUT": "-1s"},
		"zero idle timeout":         {"AUTH_HTTP_IDLE_TIMEOUT": "0s"},
		"zero shutdown timeout":     {"AUTH_HTTP_SHUTDOWN_TIMEOUT": "0s"},
		"negative shutdown timeout": {"AUTH_HTTP_SHUTDOWN_TIMEOUT": "-5s"},
		"zero access ttl":           {"AUTH_ACCESS_TOKEN_TTL": "0s"},
		"negative access ttl":       {"AUTH_ACCESS_TOKEN_TTL": "-5m"},
		"zero refresh ttl":          {"AUTH_REFRESH_TOKEN_TTL": "0s"},
		"unknown log level":         {"AUTH_LOG_LEVEL": "verbose"},
		"max conns is zero":         {"AUTH_DB_MAX_CONNS": "0"},
		"max conns is negative":     {"AUTH_DB_MAX_CONNS": "-1"},
		"min conns is negative":     {"AUTH_DB_MIN_CONNS": "-1"},
		"min conns above max conns": {"AUTH_DB_MAX_CONNS": "5", "AUTH_DB_MIN_CONNS": "6"},
		"min conns above file max":  {"AUTH_DB_MIN_CONNS": "21"},
		"zero conn lifetime":        {"AUTH_DB_MAX_CONN_LIFETIME": "0s"},
		"negative conn idle time":   {"AUTH_DB_MAX_CONN_IDLE_TIME": "-1m"},
		"zero health check period":  {"AUTH_DB_HEALTH_CHECK_PERIOD": "0s"},
		"zero connect timeout":      {"AUTH_DB_CONNECT_TIMEOUT": "0s"},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			setEnv(t, withOverrides(overrides))

			if _, err := Load(realConfigPath); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid for %v", err, overrides)
			}
		})
	}
}

// Значение не разбирается в тип поля: ошибку возвращает cleanenv, до validate.
func TestLoad_UnparsableValues(t *testing.T) {
	tests := map[string]map[string]string{
		"read timeout not a duration":    {"AUTH_HTTP_READ_TIMEOUT": "fast"},
		"access ttl not a duration":      {"AUTH_ACCESS_TOKEN_TTL": "abc"},
		"max conns not a number":         {"AUTH_DB_MAX_CONNS": "many"},
		"connect timeout not a duration": {"AUTH_DB_CONNECT_TIMEOUT": "soon"},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			setEnv(t, withOverrides(overrides))

			if _, err := Load(realConfigPath); err == nil {
				t.Fatalf("expected error for %v", overrides)
			}
		})
	}
}

func TestLogConfig_SlogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			got, err := LogConfig{Level: value}.SlogLevel()
			if err != nil {
				t.Fatalf("SlogLevel: %v", err)
			}
			if got != want {
				t.Errorf("level = %s, want %s", got, want)
			}
		})
	}
}
