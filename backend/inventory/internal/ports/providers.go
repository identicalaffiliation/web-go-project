package ports

import (
	"context"

	"go.uber.org/zap"
)

type Logger interface {
	With(fields ...zap.Field) Logger
	Debug(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	WithError(err error) Logger
	Sync() error
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
