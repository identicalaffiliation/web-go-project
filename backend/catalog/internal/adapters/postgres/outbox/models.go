package outbox

import (
	"database/sql"
	"encoding/json"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type EventStatus string

type eventModel struct { //nolint: unused
	EventID   uuid.UUID           `db:"id"`
	Key       uuid.UUID           `db:"key"`
	Payload   json.RawMessage     `db:"payload"`
	CreatedAt time.Time           `db:"created_at"`
	SentAt    sql.Null[time.Time] `db:"sent_at"`
}

func (m *eventModel) toDomain() *domain.Event {
	return &domain.Event{
		EventID:   m.EventID,
		Key:       m.Key,
		Payload:   m.Payload,
		CreatedAt: m.CreatedAt,
		SentAt:    m.SentAt.V,
	}
}
