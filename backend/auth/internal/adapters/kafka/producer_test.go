package kafka

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func twoEvents() []domain.OutboxEvent {
	return []domain.OutboxEvent{{ID: uuid.New()}, {ID: uuid.New()}}
}

func TestClassifyWriteError_PerMessageAndTransient(t *testing.T) {
	events := twoEvents()
	err := kafka.WriteErrors{kafka.MessageSizeTooLarge, kafka.LeaderNotAvailable}

	var partial *ports.PartialPublishError
	if !errors.As(classifyWriteError(events, err), &partial) {
		t.Fatal("want *PartialPublishError")
	}
	if _, ok := partial.Rejected[events[0].ID]; !ok || len(partial.Rejected) != 1 {
		t.Errorf("Rejected = %v, want only first event", partial.Rejected)
	}
	if _, ok := partial.Undelivered[events[1].ID]; !ok || len(partial.Undelivered) != 1 {
		t.Errorf("Undelivered = %v, want only second event", partial.Undelivered)
	}
}

func TestClassifyWriteError_SuccessfulMessagesInNeitherMap(t *testing.T) {
	events := twoEvents()
	err := kafka.WriteErrors{nil, kafka.MessageSizeTooLarge}

	var partial *ports.PartialPublishError
	if !errors.As(classifyWriteError(events, err), &partial) {
		t.Fatal("want *PartialPublishError")
	}
	if _, ok := partial.Rejected[events[0].ID]; ok {
		t.Error("delivered event must not be rejected")
	}
	if _, ok := partial.Undelivered[events[0].ID]; ok {
		t.Error("delivered event must not be undelivered")
	}
}

// Сетевая ошибка у каждого сообщения: ни одно не «ядовитое», попытки не тратим.
func TestClassifyWriteError_NetworkFailure_AllUndelivered(t *testing.T) {
	events := twoEvents()
	netErr := errors.New("dial tcp: connection refused")
	err := kafka.WriteErrors{netErr, netErr}

	var partial *ports.PartialPublishError
	if !errors.As(classifyWriteError(events, err), &partial) {
		t.Fatal("want *PartialPublishError")
	}
	if len(partial.Rejected) != 0 || len(partial.Undelivered) != 2 {
		t.Errorf("Rejected=%d Undelivered=%d, want 0/2", len(partial.Rejected), len(partial.Undelivered))
	}
}

// Права на топик затрагивают все сообщения, это не вина конкретного события.
func TestClassifyWriteError_TopicAuthFailed_NotRejected(t *testing.T) {
	events := twoEvents()
	err := kafka.WriteErrors{kafka.TopicAuthorizationFailed, kafka.TopicAuthorizationFailed}

	var partial *ports.PartialPublishError
	if !errors.As(classifyWriteError(events, err), &partial) {
		t.Fatal("want *PartialPublishError")
	}
	if len(partial.Rejected) != 0 {
		t.Errorf("Rejected = %v, want none", partial.Rejected)
	}
}

func TestClassifyWriteError_NotWriteErrors_GeneralFailure(t *testing.T) {
	err := classifyWriteError(twoEvents(), errors.New("context deadline exceeded"))

	var partial *ports.PartialPublishError
	if errors.As(err, &partial) {
		t.Fatal("non-WriteErrors must stay a general error")
	}
	if err == nil {
		t.Fatal("expected error")
	}
}
