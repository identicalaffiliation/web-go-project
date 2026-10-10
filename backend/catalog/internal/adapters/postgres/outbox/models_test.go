//go:build unit

package outbox

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

func Test_EventModel_ToDomain(t *testing.T) {
	id := uuid.New()
	key := uuid.New()

	model := &eventModel{
		EventID:   id,
		Key:       key,
		Payload:   json.RawMessage("some payload"),
		CreatedAt: time.Now().UTC(),
		SentAt:    sql.Null[time.Time]{V: time.Now().Add(time.Second).UTC(), Valid: true},
	}

	domainModel := model.toDomain()
	require.NotNil(t, domainModel)
	require.Equal(t, id, domainModel.EventID)
	require.Equal(t, key, domainModel.Key)
	require.Equal(t, []byte(model.Payload), domainModel.Payload)
	require.NotEmpty(t, domainModel.CreatedAt)
	require.NotEmpty(t, domainModel.SentAt)
}
