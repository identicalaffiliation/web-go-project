package ports

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"go.uber.org/zap"
)

type Logger interface {
	With(fields ...zap.Field) Logger
	Debug(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	WithError(err error) Logger
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type S3Client interface {
	GetPresignedURL(ctx context.Context, p *domain.Product) (string, error)
}
