package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/refreshtoken"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/config"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
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
		cfg.JWT.RefreshTTL,
	)

	e := serverSetup(cfg, service, log)

	// Start блокирует до остановки сервера, поэтому запускаем его в горутине,
	// а main-горутина тем временем ждёт сигнал или ошибку сервера.
	// Буфер 1: горутина отправит результат и завершится, даже если читать
	// из канала уже некому.
	errCh := make(chan error, 1)
	go func() {
		log.Info("auth service listening", "addr", cfg.HTTPAddr)
		if err := e.Start(cfg.HTTPAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down auth service")
		// Контекст ctx уже отменён сигналом, им Shutdown ограничивать нельзя:
		// он сразу бы прервал ожидание активных запросов. Нужен новый контекст
		// с собственным дедлайном.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// shutdownTimeout — сколько ждём завершения активных запросов при остановке.
const shutdownTimeout = 5 * time.Second

// serverSetup собирает echo: таймауты, middleware и маршруты. Выделено из run,
// чтобы run отвечал за жизненный цикл, а serverSetup — за то, что сервер умеет.
func serverSetup(cfg config.Config, svc ports.AuthService, log ports.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Свой http.Server не создаём: echo держит его в e.Server, и Start/Shutdown
	// работают именно с ним.
	e.Server.ReadTimeout = cfg.HTTPReadTimeout
	e.Server.WriteTimeout = cfg.HTTPWriteTimeout
	e.Server.IdleTimeout = cfg.HTTPIdleTimeout

	e.Use(recoverMiddleware(log))

	g := e.Group("/api/v1/auth")
	g.POST("/register", rest.RegisterHandler(svc, log))
	g.POST("/login", rest.LoginHandler(svc, log))
	g.POST("/refresh", rest.RefreshHandler(svc, log))
	g.POST("/logout", rest.LogoutHandler(svc, log))
	g.POST("/validate", rest.ValidateHandler(svc, log))

	return e
}

// recoverMiddleware ловит панику в хендлере, чтобы она не роняла процесс, и
// пишет её в наш JSON-лог. Свой, а не echo/middleware.Recover: тот пакет
// тянет golang.org/x/time, то есть новую зависимость.
func recoverMiddleware(log ports.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				// ErrAbortHandler — штатный способ net/http оборвать ответ.
				if r == http.ErrAbortHandler { //nolint:errorlint // сравнение со значением panic
					panic(r)
				}
				log.Error("panic recovered", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
				err = c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}()
			return next(c)
		}
	}
}
