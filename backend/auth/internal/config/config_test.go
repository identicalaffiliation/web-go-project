package config

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

var allEnvKeys = []string{
	"AUTH_DB_DSN",
	"AUTH_JWT_PRIVATE_KEY_PATH",
	"AUTH_JWT_PUBLIC_KEY_PATH",
	"AUTH_JWT_KEY_ID",
	"AUTH_JWT_ISSUER",
	"AUTH_HTTP_ADDR",
	"AUTH_ACCESS_TOKEN_TTL",
	"AUTH_REFRESH_TOKEN_TTL",
	"AUTH_LOG_LEVEL",
	"AUTH_HTTP_READ_TIMEOUT",
	"AUTH_HTTP_WRITE_TIMEOUT",
	"AUTH_HTTP_IDLE_TIMEOUT",
	"AUTH_DB_MAX_CONNS",
	"AUTH_DB_MIN_CONNS",
	"AUTH_DB_MAX_CONN_LIFETIME",
	"AUTH_DB_MAX_CONN_IDLE_TIME",
	"AUTH_DB_HEALTH_CHECK_PERIOD",
	"AUTH_DB_CONNECT_TIMEOUT",
}

// setEnv сначала полностью очищает окружение конфига, потом выставляет vars.
// t.Setenv запоминает прежнее значение и вернёт его после теста; Unsetenv
// нужен, потому что cleanenv отличает «не задана» от «задана пустой».
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
		"AUTH_DB_DSN":               "postgres://u:p@localhost:5432/auth",
		"AUTH_JWT_PRIVATE_KEY_PATH": "/keys/private.pem",
		"AUTH_JWT_PUBLIC_KEY_PATH":  "/keys/public.pem",
		"AUTH_JWT_KEY_ID":           "kid-1",
		"AUTH_JWT_ISSUER":           "auth-service",
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	for _, missing := range []string{
		"AUTH_DB_DSN",
		"AUTH_JWT_PRIVATE_KEY_PATH",
		"AUTH_JWT_PUBLIC_KEY_PATH",
		"AUTH_JWT_KEY_ID",
		"AUTH_JWT_ISSUER",
	} {
		t.Run(missing, func(t *testing.T) {
			vars := requiredVars()
			delete(vars, missing)
			setEnv(t, vars)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error when %s is not set", missing)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, requiredVars())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.JWT.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %s, want 15m", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 7*24*time.Hour {
		t.Errorf("RefreshTTL = %s, want 168h", cfg.JWT.RefreshTTL)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %s, want INFO", cfg.LogLevel)
	}
	if cfg.HTTPReadTimeout != 5*time.Second ||
		cfg.HTTPWriteTimeout != 10*time.Second ||
		cfg.HTTPIdleTimeout != 60*time.Second {
		t.Errorf("HTTP timeouts = %s/%s/%s, want 5s/10s/60s",
			cfg.HTTPReadTimeout, cfg.HTTPWriteTimeout, cfg.HTTPIdleTimeout)
	}

	wantPostgres := PostgresConfig{
		DSN:               requiredVars()["AUTH_DB_DSN"],
		MaxConns:          20,
		MinConns:          2,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}
	if cfg.Postgres != wantPostgres {
		t.Errorf("Postgres = %+v, want %+v", cfg.Postgres, wantPostgres)
	}
}

func TestLoad_LogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			vars := requiredVars()
			vars["AUTH_LOG_LEVEL"] = value
			setEnv(t, vars)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.LogLevel != want {
				t.Errorf("LogLevel = %s, want %s", cfg.LogLevel, want)
			}
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	vars := requiredVars()
	vars["AUTH_LOG_LEVEL"] = "verbose"
	setEnv(t, vars)

	if _, err := Load(); err == nil {
		t.Fatal("expected error for AUTH_LOG_LEVEL=verbose")
	}
}

func TestLoad_AllSet(t *testing.T) {
	vars := requiredVars()
	vars["AUTH_HTTP_ADDR"] = ":9090"
	vars["AUTH_ACCESS_TOKEN_TTL"] = "5m"
	vars["AUTH_REFRESH_TOKEN_TTL"] = "24h"
	vars["AUTH_DB_MAX_CONNS"] = "10"
	vars["AUTH_DB_MIN_CONNS"] = "10"
	vars["AUTH_DB_MAX_CONN_LIFETIME"] = "1h"
	vars["AUTH_DB_MAX_CONN_IDLE_TIME"] = "10m"
	vars["AUTH_DB_HEALTH_CHECK_PERIOD"] = "1m"
	vars["AUTH_DB_CONNECT_TIMEOUT"] = "3s"
	vars["AUTH_HTTP_READ_TIMEOUT"] = "2s"
	vars["AUTH_HTTP_WRITE_TIMEOUT"] = "4s"
	vars["AUTH_HTTP_IDLE_TIMEOUT"] = "30s"
	setEnv(t, vars)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Config{
		Postgres: PostgresConfig{
			DSN:               vars["AUTH_DB_DSN"],
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
			Issuer:         "auth-service",
			AccessTTL:      5 * time.Minute,
			RefreshTTL:     24 * time.Hour,
		},
		HTTPAddr:         ":9090",
		HTTPReadTimeout:  2 * time.Second,
		HTTPWriteTimeout: 4 * time.Second,
		HTTPIdleTimeout:  30 * time.Second,
		LogLevel:         slog.LevelInfo,
	}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestLoad_InvalidHTTPTimeouts(t *testing.T) {
	tests := map[string]map[string]string{
		"zero read":      {"AUTH_HTTP_READ_TIMEOUT": "0s"},
		"negative write": {"AUTH_HTTP_WRITE_TIMEOUT": "-1s"},
		"zero idle":      {"AUTH_HTTP_IDLE_TIMEOUT": "0s"},
		"not a duration": {"AUTH_HTTP_READ_TIMEOUT": "fast"},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			vars := requiredVars()
			for k, v := range overrides {
				vars[k] = v
			}
			setEnv(t, vars)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for %v", overrides)
			}
		})
	}
}

func TestLoad_InvalidTTL(t *testing.T) {
	tests := map[string]string{
		"not a duration": "abc",
		"zero":           "0s",
		"negative":       "-5m",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			vars := requiredVars()
			vars["AUTH_ACCESS_TOKEN_TTL"] = value
			setEnv(t, vars)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for AUTH_ACCESS_TOKEN_TTL=%q", value)
			}
		})
	}
}

func TestLoad_InvalidPool(t *testing.T) {
	tests := map[string]map[string]string{
		"max conns is zero":          {"AUTH_DB_MAX_CONNS": "0"},
		"max conns is negative":      {"AUTH_DB_MAX_CONNS": "-1"},
		"max conns is not a number":  {"AUTH_DB_MAX_CONNS": "many"},
		"min conns is negative":      {"AUTH_DB_MIN_CONNS": "-1"},
		"min conns above max conns":  {"AUTH_DB_MAX_CONNS": "5", "AUTH_DB_MIN_CONNS": "6"},
		"min conns above default":    {"AUTH_DB_MIN_CONNS": "21"},
		"zero conn lifetime":         {"AUTH_DB_MAX_CONN_LIFETIME": "0s"},
		"negative conn idle time":    {"AUTH_DB_MAX_CONN_IDLE_TIME": "-1m"},
		"zero health check period":   {"AUTH_DB_HEALTH_CHECK_PERIOD": "0s"},
		"zero connect timeout":       {"AUTH_DB_CONNECT_TIMEOUT": "0s"},
		"connect timeout not parsed": {"AUTH_DB_CONNECT_TIMEOUT": "soon"},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			vars := requiredVars()
			for k, v := range overrides {
				vars[k] = v
			}
			setEnv(t, vars)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for %v", overrides)
			}
		})
	}
}
