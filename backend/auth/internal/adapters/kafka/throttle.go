package kafka

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// kafkaWarnInterval — минимальный промежуток между Warn от kafka-go. Замер на
// живом брокере: при падении Kafka библиотека пишет по две строки с одним
// смыслом каждые ~5 с, это около 1800 строк в час на экземпляр.
const kafkaWarnInterval = 30 * time.Second

// warnThrottle пропускает не чаще одного Warn за interval и добавляет к
// следующему число пропущенных. Debug вместо этого скрыл бы проблему
// совсем: на уровне info оператор не увидел бы, что группа сломана.
// Побочный эффект: новая причина внутри окна тоже сворачивается в счётчик.
type warnThrottle struct {
	log      ports.Logger
	interval time.Duration
	now      func() time.Time // подменяется в тестах

	mu         sync.Mutex
	last       time.Time
	suppressed int
}

func newWarnThrottle(log ports.Logger, interval time.Duration) *warnThrottle {
	return &warnThrottle{log: log, interval: interval, now: time.Now}
}

// Printf подходит под kafka.LoggerFunc.
func (t *warnThrottle) Printf(format string, args ...any) {
	t.mu.Lock()
	now := t.now()
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		t.suppressed++
		t.mu.Unlock()
		return
	}
	suppressed := t.suppressed
	t.suppressed = 0
	t.last = now
	t.mu.Unlock()

	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if suppressed > 0 {
		t.log.Warn(msg, "suppressed", suppressed)
		return
	}
	t.log.Warn(msg)
}
