package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/labstack/echo"
)

func getFileOrNil(ctx echo.Context) (*dto.File, error) {
	header, err := ctx.FormFile("file")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return nil, echo.ErrBadRequest
	}

	if header == nil {
		return nil, nil
	}

	source, err := header.Open()
	if err != nil {
		return nil, echo.ErrInternalServerError
	}

	defer func() {
		if err := source.Close(); err != nil {
			slog.Error("failed to close source file", "error", err)
		}
	}()

	return dto.NewFile(source, header), nil
}
