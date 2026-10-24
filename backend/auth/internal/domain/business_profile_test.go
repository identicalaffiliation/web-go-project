package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

const (
	eventID = "019a0000-0000-7000-8000-000000000009"
	userID  = "019a0000-0000-7000-8000-000000000001"
)

func TestParseBusinessProfileCreated(t *testing.T) {
	raw := `{"event_id":"` + eventID + `","user_id":"` + userID + `","occurred_at":"2026-01-01T00:00:00Z"}`

	ev, err := domain.ParseBusinessProfileCreated([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.UserID != uuid.MustParse(userID) || ev.EventID != uuid.MustParse(eventID) {
		t.Errorf("unexpected event: %+v", ev)
	}
}

// Лишние поля контракта не должны ломать разбор.
func TestParseBusinessProfileCreated_ExtraFields(t *testing.T) {
	raw := `{"event_id":"` + eventID + `","user_id":"` + userID + `","company_name":"ACME"}`

	_, err := domain.ParseBusinessProfileCreated([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBusinessProfileCreated_Invalid(t *testing.T) {
	tests := map[string]string{
		"broken json":      `{"user_id":`,
		"not an object":    `[]`,
		"empty body":       ``,
		"missing user_id":  `{"event_id":"` + eventID + `"}`,
		"nil user_id":      `{"event_id":"` + eventID + `","user_id":"00000000-0000-0000-0000-000000000000"}`,
		"user_id not uuid": `{"event_id":"` + eventID + `","user_id":"abc"}`,
		"missing event_id": `{"user_id":"` + userID + `"}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := domain.ParseBusinessProfileCreated([]byte(raw))
			if !errors.Is(err, domain.ErrInvalidEvent) {
				t.Fatalf("want ErrInvalidEvent, got %v", err)
			}
		})
	}
}
