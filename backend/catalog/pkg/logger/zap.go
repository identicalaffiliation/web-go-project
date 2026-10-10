package logger

import (
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"go.uber.org/zap"
)

type Logger struct {
	logger *zap.Logger
}

func ProvideLogger() (ports.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	return &Logger{logger: logger}, nil
}

func (l *Logger) WithError(err error) ports.Logger {
	return l.With(zap.Error(err))
}

func (l *Logger) With(fields ...zap.Field) ports.Logger {
	return &Logger{
		l.logger.With(fields...),
	}
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

func (l *Logger) Sync() error {
	return l.logger.Sync()
}
