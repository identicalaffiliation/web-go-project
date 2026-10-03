package rest

import (
	"fmt"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rest/handlers"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func SetupServer(cfg *config.HTTPConfig, c *catalog.Service) *echo.Echo {
	e := echo.New()
	e.Server.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	e.Server.ReadTimeout = cfg.ReadTimeout
	e.Server.WriteTimeout = cfg.WriteTimeout
	e.Server.IdleTimeout = cfg.IdleTimeout

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	baseAPI := e.Group("/api/v1")

	internal := baseAPI.Group("/catalog")
	internal.POST("", handlers.AddItemToCatalog(c))
	internal.GET("/:productId", handlers.GetItem(c))
	internal.GET("", handlers.GetItemsPage(c))

	return e
}
