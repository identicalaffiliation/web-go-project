package outbox

import (
	"database/sql"
	"encoding/json"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

const (
	Pending    EventStatus = "pending"
	Processing EventStatus = "processing"
	Sent       EventStatus = "sent"
)

type EventStatus string

type eventModels []eventModel //nolint: unused

func (em *eventModels) toDomainList() []*domain.Event { //nolint: unused
	list := make([]*domain.Event, 0, len(*em))
	for _, m := range *em {
		list = append(list, m.toDomain())
	}
	return list
}

type eventModel struct { //nolint: unused
	EventID   uuid.UUID           `db:"id"`
	Key       uuid.UUID           `db:"key"`
	Payload   json.RawMessage     `db:"payload"`
	CreatedAt time.Time           `db:"created_at"`
	SentAt    sql.Null[time.Time] `db:"sent_at"`
}

func (m *eventModel) toDomain() *domain.Event { //nolint: unused
	return &domain.Event{
		EventID:   m.EventID,
		Key:       m.Key,
		Payload:   m.Payload,
		CreatedAt: m.CreatedAt,
		SentAt:    m.SentAt.V,
	}
}
