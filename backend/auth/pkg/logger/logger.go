package logger

import (
	"io"
	"log/slog"
	"os"
)

// Logger — тонкая обёртка над slog с цепочечными With/WithError.
// Методы With* возвращают новый логгер и не меняют исходный, поэтому
// им безопасно пользоваться из нескольких горутин.
type Logger struct {
	l *slog.Logger
}

// New создаёт JSON-логгер в stdout с заданным минимальным уровнем.
func New(level slog.Level) *Logger {
	return NewWithWriter(os.Stdout, level)
}

// NewWithWriter нужен тестам, чтобы читать вывод логгера из буфера.
func NewWithWriter(w io.Writer, level slog.Level) *Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return &Logger{l: slog.New(h)}
}

func (l *Logger) Debug(msg string, args ...any) { l.l.Debug(msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.l.Info(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.l.Warn(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.l.Error(msg, args...) }

// With возвращает логгер, который добавляет args к каждой записи.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{l: l.l.With(args...)}
}

// WithError добавляет ошибку в поле "error". Для nil возвращает тот же
// логгер: запись "error":null только сбивала бы при чтении логов.
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.With("error", err)
}
