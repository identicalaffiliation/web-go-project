package logging

import "github.com/identicalaffiliation/web-go-project/auth/internal/ports"

type nop struct{}

var _ ports.Logger = nop{}

// NewNop возвращает логгер, который ничего не пишет. Нужен тестам, чтобы
// не расписывать мок логгера руками.
func NewNop() ports.Logger { return nop{} }

func (nop) Debug(string, ...any)           {}
func (nop) Info(string, ...any)            {}
func (nop) Warn(string, ...any)            {}
func (nop) Error(string, ...any)           {}
func (n nop) With(...any) ports.Logger     { return n }
func (n nop) WithError(error) ports.Logger { return n }
