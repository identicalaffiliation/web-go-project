package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
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

	parent := context.Background()

	notifyCtx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	for range cfg.Kafka.Workers {
		wg.Go(func() {
			consumer := kafka.NewConsumer(cfg, logger)
			defer func() {
				if err := consumer.Close(); err != nil {
					logger.WithError(err).Error("failed to close consumer")
				}
			}()

			consumer.ReadMessage(notifyCtx)
		})
	}
	
	wg.Wait()
}
