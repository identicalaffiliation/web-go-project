package main

import (
	"context"
	"flag"
	"os"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/box"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/payments/pkg/logger"
	"github.com/identicalaffiliation/web-go-project/backend/payments/pkg/shortcut"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	log, err := logger.ProvideLogger()
	shortcut.ErrNotNilFatal(err)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.WithError(err).Error("failed to load config")
		os.Exit(1)
	}

	if err := box.ProvidePaymentsBox(context.Background(), cfg, log); err != nil {
		log.WithError(err).Error("payments service stopped with error")
		os.Exit(1)
	}
}
