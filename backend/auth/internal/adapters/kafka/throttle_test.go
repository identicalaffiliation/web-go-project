package kafka

import (
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type warnRecord struct {
	msg  string
	args []any
}

// warnCapture запоминает Warn с аргументами; остальные уровни не нужны.
type warnCapture struct{ warns *[]warnRecord }

func (c warnCapture) Debug(string, ...any) {}
func (c warnCapture) Info(string, ...any)  {}
func (c warnCapture) Error(string, ...any) {}
func (c warnCapture) Warn(msg string, args ...any) {
	*c.warns = append(*c.warns, warnRecord{msg: msg, args: args})
}
func (c warnCapture) With(...any) ports.Logger     { return c }
func (c warnCapture) WithError(error) ports.Logger { return c }

func TestWarnThrottle(t *testing.T) {
	var warns []warnRecord
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	th := newWarnThrottle(warnCapture{&warns}, 30*time.Second)
	th.now = func() time.Time { return now }

	th.Printf("dial %s: refused\n", "broker") // первый проходит сразу
	now = now.Add(5 * time.Second)
	th.Printf("again")
	now = now.Add(20 * time.Second)
	th.Printf("again")
	if len(warns) != 1 {
		t.Fatalf("warns inside the interval = %d, want 1", len(warns))
	}
	if warns[0].msg != "dial broker: refused" {
		t.Errorf("msg = %q, want trimmed formatted message", warns[0].msg)
	}
	if len(warns[0].args) != 0 {
		t.Errorf("first warn has args %v, want none", warns[0].args)
	}

	now = now.Add(10 * time.Second) // 35 с с первого сообщения
	th.Printf("after the interval")
	if len(warns) != 2 {
		t.Fatalf("warns after the interval = %d, want 2", len(warns))
	}
	if got := warns[1].args; len(got) != 2 || got[0] != "suppressed" || got[1] != 2 {
		t.Errorf("args = %v, want [suppressed 2]", got)
	}

	// Счётчик сброшен: следующее сообщение после паузы идёт без suppressed.
	now = now.Add(31 * time.Second)
	th.Printf("third window")
	if len(warns) != 3 || len(warns[2].args) != 0 {
		t.Errorf("third window: warns=%d args=%v, want a clean third message", len(warns), warns[len(warns)-1].args)
	}
}
