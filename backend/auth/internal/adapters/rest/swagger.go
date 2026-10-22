package rest

import (
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"

	// Побочный импорт регистрирует сгенерированную спецификацию в swag.
	_ "github.com/identicalaffiliation/web-go-project/auth/docs/auth"
)

// RegisterSwagger вешает Swagger UI на группу: при префиксе /api/v1/auth он
// открывается по /api/v1/auth/swagger/index.html. Путь внутри /api/v1/auth/
// выбран потому, что nginx уже проксирует этот префикс в auth.
func RegisterSwagger(g *echo.Group) {
	g.GET("/swagger/*", echoSwagger.WrapHandler)
}
