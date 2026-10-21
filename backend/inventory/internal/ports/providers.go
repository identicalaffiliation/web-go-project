package ports

import "go.uber.org/zap"

type Logger interface {
	With(fields ...zap.Field) Logger
	Debug(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	WithError(err error) Logger
	Sync() error
}
