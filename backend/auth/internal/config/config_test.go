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
	setEnv(t, vars)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Config{
		Postgres: PostgresConfig{DSN: vars["AUTH_DB_DSN"]},
		JWT: JWTConfig{
			PrivateKeyPath: "/keys/private.pem",
			PublicKeyPath:  "/keys/public.pem",
			KeyID:          "kid-1",
			Issuer:         "auth-service",
			AccessTTL:      5 * time.Minute,
			RefreshTTL:     24 * time.Hour,
		},
		HTTPAddr: ":9090",
		LogLevel: slog.LevelInfo,
	}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
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
