//go:build unit

package logger

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedLogger(level zapcore.Level) (*Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &Logger{logger: zap.New(core)}, logs
}

func TestProvideLogger(t *testing.T) {
	l, err := ProvideLogger()
	require.NoError(t, err)
	require.NotNil(t, l)
}

func TestLogger(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		l, logs := newObservedLogger(zapcore.DebugLevel)

		l.Error("boom", zap.String("key", "value"))

		entries := logs.All()
		require.Len(t, entries, 1)
		require.Equal(t, zapcore.ErrorLevel, entries[0].Level)
		require.Equal(t, "boom", entries[0].Message)
		require.Equal(t, "value", entries[0].ContextMap()["key"])
	})

	t.Run("debug", func(t *testing.T) {
		l, logs := newObservedLogger(zapcore.DebugLevel)

		l.Debug("dbg")

		entries := logs.All()
		require.Len(t, entries, 1)
		require.Equal(t, zapcore.DebugLevel, entries[0].Level)
		require.Equal(t, "dbg", entries[0].Message)
	})

	t.Run("debug filtered by level", func(t *testing.T) {
		l, logs := newObservedLogger(zapcore.InfoLevel)

		l.Debug("dbg")

		require.Zero(t, logs.Len())
	})

	t.Run("with error", func(t *testing.T) {
		l, logs := newObservedLogger(zapcore.DebugLevel)

		l.WithError(errors.New("some error")).Error("failed")

		entries := logs.All()
		require.Len(t, entries, 1)
		require.Equal(t, "some error", entries[0].ContextMap()["error"])
	})

	t.Run("with does not mutate parent", func(t *testing.T) {
		l, logs := newObservedLogger(zapcore.DebugLevel)

		child := l.With(zap.String("child", "yes"))
		require.NotSame(t, l, child)

		child.Error("from child")
		l.Error("from parent")

		entries := logs.All()
		require.Len(t, entries, 2)
		require.Equal(t, "yes", entries[0].ContextMap()["child"])
		require.NotContains(t, entries[1].ContextMap(), "child")
	})

	t.Run("sync", func(t *testing.T) {
		l, _ := newObservedLogger(zapcore.DebugLevel)
		require.NoError(t, l.Sync())
	})
}
