package logging

import (
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/logger"
)

type adapter struct {
	l *logger.Logger
}

var _ ports.Logger = (*adapter)(nil)

func New(l *logger.Logger) ports.Logger {
	return &adapter{l: l}
}

func (a *adapter) Debug(msg string, args ...any) { a.l.Debug(msg, args...) }
func (a *adapter) Info(msg string, args ...any)  { a.l.Info(msg, args...) }
func (a *adapter) Warn(msg string, args ...any)  { a.l.Warn(msg, args...) }
func (a *adapter) Error(msg string, args ...any) { a.l.Error(msg, args...) }

func (a *adapter) With(args ...any) ports.Logger {
	return &adapter{l: a.l.With(args...)}
}

func (a *adapter) WithError(err error) ports.Logger {
	return &adapter{l: a.l.WithError(err)}
}
