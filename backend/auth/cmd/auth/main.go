package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/clock"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/idgen"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/refreshtoken"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/config"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/hasher"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/jwt"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		// Конфиг мог не загрузиться, поэтому логгера по уровню из него ещё нет:
		// сообщение о причине остановки пишем временным логгером с уровнем по умолчанию.
		logging.New(logger.New(slog.LevelInfo)).WithError(err).Error("auth service stopped")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logging.New(logger.New(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{
		DSN:               cfg.Postgres.DSN,
		MaxConns:          cfg.Postgres.MaxConns,
		MinConns:          cfg.Postgres.MinConns,
		MaxConnLifetime:   cfg.Postgres.MaxConnLifetime,
		MaxConnIdleTime:   cfg.Postgres.MaxConnIdleTime,
		HealthCheckPeriod: cfg.Postgres.HealthCheckPeriod,
		ConnectTimeout:    cfg.Postgres.ConnectTimeout,
	})
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

	issuer := jwt.NewIssuer(privateKey, cfg.JWT.KeyID, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	verifier := jwt.NewVerifier(publicKey, cfg.JWT.Issuer)

	service := app.NewService(
		postgres.NewUserRepository(pool),
		postgres.NewRefreshTokenRepository(pool),
		hasher.New(),
		issuer,
		verifier,
		refreshtoken.New(),
		clock.New(),
		idgen.New(),
		cfg.JWT.RefreshTTL,
	)

	e := echo.New()
	e.HideBanner = true
	rest.NewHandler(service, log).Register(e.Group("/api/v1/auth"))

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      e,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("auth service listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down auth service")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
