package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/clock"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/idgen"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/refreshtoken"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/config"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/hasher"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/jwt"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("auth service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	privateKeyBytes, err := os.ReadFile(cfg.JWT.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}
	privateKey, err := jwt.ParsePrivateKeyPEM(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	publicKeyBytes, err := os.ReadFile(cfg.JWT.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	publicKey, err := jwt.ParsePublicKeyPEM(publicKeyBytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	issuer := jwt.NewIssuer(privateKey, cfg.JWT.KeyID, cfg.JWT.Issuer, app.AccessTokenTTL)
	verifier := jwt.NewVerifier(publicKey, cfg.JWT.Issuer)

	service := app.NewService(
		postgres.NewUserRepository(pool),
		postgres.NewRefreshTokenRepository(pool),
		hasher.New(hasher.DefaultCost),
		issuer,
		verifier,
		refreshtoken.New(),
		clock.New(),
		idgen.New(),
	)
	_ = service // REST/gRPC слой подключим следующим шагом

	logger.Info("auth service initialized", "http_addr", cfg.HTTPAddr)
	return nil
}
