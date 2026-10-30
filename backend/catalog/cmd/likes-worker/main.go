package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/box"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/shortcut"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	notifyCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	zapLogger, err := logger.ProvideLogger()
	shortcut.ErrNotNilFatal(err)

	cfg, err := config.ProvideConfig(configPath)
	shortcut.ErrNotNilFatal(err)

	if err := box.ProvideLikesWorker(notifyCtx, cfg, zapLogger); err != nil {
		zapLogger.WithError(err).Error("failed to run app")
		os.Exit(1)
	}

	os.Exit(0)
}
