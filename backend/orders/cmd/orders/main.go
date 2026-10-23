package main

import (
	"context"
	"flag"
	"os"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/box"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/orders/pkg/logger"
	"github.com/identicalaffiliation/web-go-project/backend/orders/pkg/shortcut"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	ctx := context.Background()

	log, err := logger.ProvideLogger()
	shortcut.ErrNotNilFatal(err)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.WithError(err).Error("failed to load config")
		os.Exit(1)
	}

	if err := box.ProvideOrdersBox(ctx, cfg, log); err != nil {
		log.WithError(err).Error("failed to run application")
	}

	os.Exit(0)
}
