package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type OutboxWorkerConfig struct {
	Interval       time.Duration
	BatchSize      int
	PublishTimeout time.Duration
	MaxAttempts    int
}

// OutboxWorker переносит события из таблицы outbox в Kafka (at-least-once).
type OutboxWorker struct {
	store     ports.OutboxStore
	publisher ports.EventPublisher
	log       ports.Logger
	cfg       OutboxWorkerConfig
}

func NewOutboxWorker(
	store ports.OutboxStore,
	publisher ports.EventPublisher,
	log ports.Logger,
	cfg OutboxWorkerConfig,
) *OutboxWorker {
	return &OutboxWorker{store: store, publisher: publisher, log: log, cfg: cfg}
}

// Run блокируется до отмены ctx. Запускается в отдельной горутине: вызывающий
// ждёт её завершения (WaitGroup/errgroup), прежде чем закрывать producer и пул.
func (w *OutboxWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Штатная остановка: ошибкой не считается. Сюда мы попадаем только
			// между батчами, поэтому текущий батч уже завершён.
			return nil
		case <-ticker.C:
			// Батч не должен прерываться отменой ctx посреди публикации: иначе
			// сообщения могли бы уйти в Kafka, а sent_at не записаться. Поэтому
			// отвязываем контекст от отмены, а время ограничиваем таймаутом.
			batchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.cfg.PublishTimeout)
			err := w.processBatch(batchCtx)
			cancel()
			if err != nil {
				// Не выходим: временная недоступность Kafka или БД не должна
				// останавливать воркер, следующий тик повторит попытку.
				w.log.WithError(err).Error("outbox batch failed")
			}
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) (err error) {
	batch, err := w.store.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin batch: %w", err)
	}
	// Незавершённая транзакция откатывается: блокировки строк снимаются.
	defer func() {
		if rbErr := batch.Rollback(ctx); rbErr != nil {
			err = errors.Join(err, rbErr)
		}
	}()

	events, err := batch.Fetch(ctx, w.cfg.BatchSize, w.cfg.MaxAttempts)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	var (
		sent     []uuid.UUID
		rejected []domain.OutboxEvent
	)

	pubErr := w.publisher.Publish(ctx, events)
	switch partial := (*ports.PartialPublishError)(nil); {
	case pubErr == nil:
		sent = eventIDs(events)
	case errors.As(pubErr, &partial):
		sent, rejected = splitPartial(events, partial)
		if len(sent) == 0 && len(rejected) == 0 {
			// Не дошло ничего и не по вине сообщений: это общий сбой.
			return w.generalFailure(pubErr, len(events))
		}
		w.log.Warn("outbox batch delivered partially",
			"sent", len(sent), "rejected", len(rejected), "undelivered", len(partial.Undelivered))
	default:
		return w.generalFailure(pubErr, len(events))
	}

	if len(sent) > 0 {
		if err := batch.MarkSent(ctx, sent); err != nil {
			return fmt.Errorf("mark sent: %w", err)
		}
	}
	if len(rejected) > 0 {
		// Счётчик нужно сохранить, поэтому дальше транзакция коммитится, а не
		// откатывается. Попытка засчитывается только отклонённым сообщениям.
		if err := batch.MarkFailed(ctx, eventIDs(rejected)); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
	}
	if err := batch.Commit(ctx); err != nil {
		// События уже в Kafka, а sent_at не сохранён: на следующем тике уйдут
		// повторно. Это и есть at-least-once, консьюмер дедуплицирует по event_id.
		return fmt.Errorf("commit batch: %w", err)
	}

	w.logExhausted(rejected, partialRejected(pubErr))
	w.log.Info("outbox batch processed", "sent", len(sent), "rejected", len(rejected))
	return nil
}

// generalFailure обрабатывает сбой без поэлементных ошибок (Kafka недоступна,
// таймаут): попытки не засчитываются, а транзакция откатывается в defer, так
// что события спокойно повторятся на следующем тике. Иначе простой брокера
// исчерпал бы max_attempts у всех событий.
func (w *OutboxWorker) generalFailure(err error, count int) error {
	w.log.WithError(err).Warn("outbox publish failed, will retry on next tick", "count", count)
	return nil
}

// logExhausted пишет один Error на событие, исчерпавшее попытки: после этого
// Fetch его больше не выбирает, и повторных логов не будет. Payload не
// логируем: в нём email, то есть персональные данные.
func (w *OutboxWorker) logExhausted(rejected []domain.OutboxEvent, reasons map[uuid.UUID]error) {
	for _, e := range rejected {
		attempts := e.Attempts + 1 // в базе уже +1, в e лежит значение до обновления
		if attempts < w.cfg.MaxAttempts {
			continue
		}
		w.log.WithError(reasons[e.ID]).Error("outbox event reached max attempts",
			"event_id", e.ID, "topic", e.Topic, "attempts", attempts)
	}
}

// splitPartial делит батч на доставленные и отклонённые брокером события;
// недоставленные по причине связи не попадают ни туда, ни туда.
func splitPartial(events []domain.OutboxEvent, p *ports.PartialPublishError) (sent []uuid.UUID, rejected []domain.OutboxEvent) {
	for _, e := range events {
		if _, ok := p.Rejected[e.ID]; ok {
			rejected = append(rejected, e)
			continue
		}
		if _, ok := p.Undelivered[e.ID]; ok {
			continue
		}
		sent = append(sent, e.ID)
	}
	return sent, rejected
}

func partialRejected(err error) map[uuid.UUID]error {
	var p *ports.PartialPublishError
	if errors.As(err, &p) {
		return p.Rejected
	}
	return nil
}

func eventIDs(events []domain.OutboxEvent) []uuid.UUID {
	ids := make([]uuid.UUID, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}
	return ids
}
