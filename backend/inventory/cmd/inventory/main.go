package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/postgres/stock"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/temporal"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
	taskconsumer "github.com/identicalaffiliation/web-go-project/backend/inventory/internal/service/task-consumer"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/service/workflow"
	logger2 "github.com/identicalaffiliation/web-go-project/backend/inventory/pkg/logger"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/pkg/shortcut"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	logger, err := logger2.ProvideLogger()
	shortcut.ErrNotNilFatal(err)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.WithError(err).Error("failed to load config")
		os.Exit(1)
	}

	if err := run(cfg, logger); err != nil {
		logger.WithError(err).Error("inventory service stopped with error")
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger ports.Logger) error {
	notifyCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := getPostgresPool(notifyCtx, cfg.OperationDuration, cfg.DB.MasterDSN, &cfg.DB.Master)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := stock.NewRepository(pool)
	taskProcesser := taskconsumer.NewTaskProcesser(repo)
	workflowService := workflow.NewService(repo, logger, manager.Must(trm.NewDefaultFactory(pool)))

	temporalWorker, err := temporal.NewWorker(&cfg.Temporal, temporal.NewActivities(workflowService))
	if err != nil {
		return err
	}
	defer temporalWorker.Close()

	var wg sync.WaitGroup

	for range cfg.Kafka.Workers {
		wg.Go(func() {
			consumer := kafka.NewConsumer(cfg, logger, taskProcesser)
			defer func() {
				if err := consumer.Close(); err != nil {
					logger.WithError(err).Error("failed to close consumer")
				}
			}()

			consumer.ReadMessage(notifyCtx)
		})
	}

	var workerErr error
	wg.Go(func() {
		if workerErr = temporalWorker.Run(notifyCtx.Done()); workerErr != nil {
			stop()
		}
	})

	wg.Wait()

	return workerErr
}

func getPostgresPool(
	ctx context.Context,
	duration time.Duration,
	dsn string,
	cfg *config.PoolConfig,
) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(dsn, cfg)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeoutCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

// poolConfig - pgxpool.Config из DSN с настройками пула из конфига.
// Нулевые поля конфига не трогают значения pgx по умолчанию (или pool_* из DSN).
func poolConfig(dsn string, cfg *config.PoolConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if poolCfg.MinConns > poolCfg.MaxConns {
		return nil, fmt.Errorf("postgres pool: min_conns (%d) > max_conns (%d)", poolCfg.MinConns, poolCfg.MaxConns)
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}
	// таймаут на каждое новое соединение, а не только на первое при старте
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	return poolCfg, nil
}
