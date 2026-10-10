//go:build integration

package integration

import (
	"bytes"
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestOutboxSuite(t *testing.T) {
	suite.Run(t, new(DatabaseSuite))
}

func (s *DatabaseSuite) TestOutboxRepository_Insert() {
	payload := `{"payload": "some payload"}`
	event := &domain.Event{
		EventID: uuid.New(),
		Key:     uuid.New(),
		Payload: bytes.NewBufferString(payload).Bytes(),
	}

	s.Require().NoError(s.outbox.Insert(s.ctx, event))
}

func (s *DatabaseSuite) TestOutboxRepository_GetEvents() {
	t := s.T()

	events := make([]*domain.Event, 0, 3)
	for i := 0; i < 3; i++ {
		event := &domain.Event{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"test"}`),
		}
		s.Require().NoError(s.outbox.Insert(s.ctx, event))
		events = append(events, event)
	}

	s.Require().NoError(s.outbox.SentEvent(s.ctx, []uuid.UUID{events[0].EventID}))

	got, err := s.outbox.GetEvents(s.ctx, 10)
	s.Require().NoError(err)
	require.Len(t, got, 2)

	gotIDs := make(map[uuid.UUID]bool, len(got))
	for _, e := range got {
		gotIDs[e.EventID] = true
	}

	require.False(t, gotIDs[events[0].EventID], "sent event should not be returned")
	require.True(t, gotIDs[events[1].EventID], "unsent event should be returned")
	require.True(t, gotIDs[events[2].EventID], "unsent event should be returned")

	for _, e := range got {
		require.NotEmpty(t, e.EventID)
		require.NotEmpty(t, e.Key)
		require.NotEmpty(t, e.Payload)
		require.NotEmpty(t, e.CreatedAt)
		require.True(t, e.SentAt.IsZero(), "returned events should have zero SentAt")
	}
}

func (s *DatabaseSuite) TestOutboxRepository_GetEvents_Limit() {
	t := s.T()

	for i := 0; i < 5; i++ {
		event := &domain.Event{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"test"}`),
		}
		s.Require().NoError(s.outbox.Insert(s.ctx, event))
	}

	got, err := s.outbox.GetEvents(s.ctx, 2)
	s.Require().NoError(err)
	require.Len(t, got, 2)
}

func (s *DatabaseSuite) TestOutboxRepository_GetEvents_Empty() {
	t := s.T()

	got, err := s.outbox.GetEvents(s.ctx, 10)
	s.Require().NoError(err)
	require.Empty(t, got)
}

func (s *DatabaseSuite) TestOutboxRepository_SentEvent() {
	t := s.T()

	ids := make([]uuid.UUID, 0, 3)
	for i := 0; i < 3; i++ {
		event := &domain.Event{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"test"}`),
		}
		s.Require().NoError(s.outbox.Insert(s.ctx, event))
		ids = append(ids, event.EventID)
	}

	s.Require().NoError(s.outbox.SentEvent(s.ctx, ids[:2]))

	got, err := s.outbox.GetEvents(s.ctx, 10)
	s.Require().NoError(err)
	require.Len(t, got, 1)
	require.Equal(t, ids[2], got[0].EventID)
}

func (s *DatabaseSuite) TestOutboxRepository_SentEvent_EmptyIDs() {
	s.Require().NoError(s.outbox.SentEvent(s.ctx, []uuid.UUID{}))
}

func (s *DatabaseSuite) TestOutboxRepository_SentEvent_AllSent() {
	t := s.T()

	ids := make([]uuid.UUID, 0, 3)
	for i := 0; i < 3; i++ {
		event := &domain.Event{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"test"}`),
		}
		s.Require().NoError(s.outbox.Insert(s.ctx, event))
		ids = append(ids, event.EventID)
	}

	s.Require().NoError(s.outbox.SentEvent(s.ctx, ids))

	got, err := s.outbox.GetEvents(s.ctx, 10)
	s.Require().NoError(err)
	require.Empty(t, got)
}
