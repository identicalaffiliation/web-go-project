package ports

//go:generate go tool mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks

// Logger — порт логирования. With и WithError возвращают ports.Logger, а не
// конкретный тип, поэтому реализация подключается через адаптер
// (internal/adapters/logging), а не напрямую из pkg/logger.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithError(err error) Logger
}
