package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/box"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/shortcut"
)

// @title Shop catalog service API
// @version 1.0
// @description Service for shop catalog: creating for business and parsing for customers.
// @host localhost
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT.
func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	zapLogger, err := logger.ProvideLogger()
	shortcut.ErrNotNilFatal(err)

	defer func() {
		if err := zapLogger.Sync(); err != nil {
			fmt.Println(err)
		}
	}()

	cfg, err := config.ProvideConfig(configPath)
	if err != nil {
		zapLogger.WithError(err).Error("provide config error")
		os.Exit(1)
	}

	ctx := context.Background()
	if err := box.SetupCatalogBox(ctx, cfg, zapLogger); err != nil {
		zapLogger.WithError(err).Error("failed to setup box")
		os.Exit(1)
	}

	zapLogger.Debug("app was stopped gracefully..")
}
