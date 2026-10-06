//go:build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"uuid"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

func TestKafkaSuite(t *testing.T) {
	suite.Run(t, new(KafkaSuite))
}

func (s *KafkaSuite) Test_Producer_SendMessages_SingleEvent() {
	ctx := context.Background()
	reader := s.newReader("test-group-single-" + uuid.New().String())
	defer reader.Close()

	eventID := uuid.New()
	key := uuid.New()
	payload := []byte(`{"event":"created","data":"single"}`)

	event := &domain.Event{
		EventID: eventID,
		Key:     key,
		Payload: payload,
	}

	s.Require().NoError(s.producer.SendMessages(ctx, []*domain.Event{event}))

	readCtx, cancel := context.WithTimeout(ctx, readWait)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	s.Require().NoError(err)

	s.Equal(payload, msg.Value)
	s.Equal(key.String(), string(msg.Key))
	s.Require().Len(msg.Headers, 1)
	s.Equal("event_id", msg.Headers[0].Key)
	s.Equal(eventID.String(), string(msg.Headers[0].Value))
}

func (s *KafkaSuite) Test_Producer_SendMessages_MultipleEvents() {
	ctx := context.Background()
	reader := s.newReader("test-group-multi-" + uuid.New().String())
	defer reader.Close()

	events := []*domain.Event{
		{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"created","data":"first"}`),
		},
		{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"created","data":"second"}`),
		},
		{
			EventID: uuid.New(),
			Key:     uuid.New(),
			Payload: []byte(`{"event":"created","data":"third"}`),
		},
	}

	s.Require().NoError(s.producer.SendMessages(ctx, events))

	readCtx, cancel := context.WithTimeout(ctx, readWait)
	defer cancel()

	for i, expected := range events {
		msg, err := reader.ReadMessage(readCtx)
		s.Require().NoError(err, "failed to read message #%d", i)

		s.Equal(expected.Payload, msg.Value)
		s.Equal(expected.Key.String(), string(msg.Key))
		s.Require().Len(msg.Headers, 1)
		s.Equal("event_id", msg.Headers[0].Key)
		s.Equal(expected.EventID.String(), string(msg.Headers[0].Value))
	}
}

func (s *KafkaSuite) Test_Producer_SendMessages_EmptyBatch() {
	ctx := context.Background()

	err := s.producer.SendMessages(ctx, []*domain.Event{})
	s.Require().NoError(err)
}

func (s *KafkaSuite) Test_Producer_SendMessages_BinaryPayload() {
	ctx := context.Background()
	reader := s.newReader("test-group-binary-" + uuid.New().String())
	defer reader.Close()

	payload := bytes.Repeat([]byte{0xAB, 0xCD, 0xEF, 0x00, 0x01}, 100)

	event := &domain.Event{
		EventID: uuid.New(),
		Key:     uuid.New(),
		Payload: payload,
	}

	s.Require().NoError(s.producer.SendMessages(ctx, []*domain.Event{event}))

	readCtx, cancel := context.WithTimeout(ctx, readWait)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	s.Require().NoError(err)
	s.Equal(payload, msg.Value)
}

func (s *KafkaSuite) Test_Producer_SendMessages_PreservesEventIDHeader() {
	ctx := context.Background()
	reader := s.newReader("test-group-header-" + uuid.New().String())
	defer reader.Close()

	eventID := uuid.New()
	event := &domain.Event{
		EventID: eventID,
		Key:     uuid.New(),
		Payload: []byte(`{"event":"header-test"}`),
	}

	s.Require().NoError(s.producer.SendMessages(ctx, []*domain.Event{event}))

	readCtx, cancel := context.WithTimeout(ctx, readWait)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	s.Require().NoError(err)

	var hdr kafka.Header
	for _, h := range msg.Headers {
		if h.Key == "event_id" {
			hdr = h
			break
		}
	}

	s.Equal("event_id", hdr.Key)
	s.Equal(eventID.String(), string(hdr.Value))
}
