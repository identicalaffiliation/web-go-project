//go:build integration

package integration

import (
	"bytes"
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/suite"
)

func TestOutboxSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) TestOutboxRepository_Insert() {
	payload := `{"payload": "some payload"}`
	event := &domain.Event{
		EventID: uuid.New(),
		Key:     uuid.New(),
		Payload: bytes.NewBufferString(payload).Bytes(),
	}

	s.Require().NoError(s.outbox.Insert(s.ctx, event))
}
