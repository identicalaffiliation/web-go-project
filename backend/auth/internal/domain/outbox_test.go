package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

// Golden-тест контракта users.created: Profile (Python) читает тело по именам
// полей, поэтому переименование ключа должно ломать CI, а не продакшен.
func TestNewUserCreatedEvent_PayloadContract(t *testing.T) {
	user := domain.User{
		ID:    uuid.MustParse("0190a000-0000-7000-8000-000000000001"),
		Email: "alice@example.com",
		Role:  domain.RoleBusiness,
	}
	eventID := uuid.MustParse("0190a000-0000-7000-8000-000000000002")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	event, err := domain.NewUserCreatedEvent(user, eventID, "users.created", now)
	if err != nil {
		t.Fatalf("NewUserCreatedEvent: %v", err)
	}

	// Декодируем в map, а не в структуру: так проверяются именно ключи JSON,
	// а не теги, которые тест иначе подтвердил бы сам собой.
	var body map[string]any
	if err := json.Unmarshal(event.Payload, &body); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	for _, key := range []string{"event_id", "user_id", "email", "role", "occurred_at"} {
		if _, ok := body[key]; !ok {
			t.Errorf("payload has no %q key: %s", key, event.Payload)
		}
	}

	assertUUIDField(t, body, "event_id", eventID)
	assertUUIDField(t, body, "user_id", user.ID)

	if got := body["email"]; got != "alice@example.com" {
		t.Errorf("email = %v, want alice@example.com", got)
	}
	if got := body["role"]; got != "business" {
		t.Errorf("role = %v, want business", got)
	}

	raw, ok := body["occurred_at"].(string)
	if !ok {
		t.Fatalf("occurred_at is %T, want string", body["occurred_at"])
	}
	occurredAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("occurred_at %q is not RFC3339: %v", raw, err)
	}
	if !occurredAt.Equal(now) {
		t.Errorf("occurred_at = %v, want %v", occurredAt, now)
	}
}

func assertUUIDField(t *testing.T, body map[string]any, key string, want uuid.UUID) {
	t.Helper()

	raw, ok := body[key].(string)
	if !ok {
		t.Fatalf("%s is %T, want string", key, body[key])
	}
	got, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("%s %q is not a UUID: %v", key, raw, err)
	}
	if got != want {
		t.Errorf("%s = %s, want %s", key, got, want)
	}
}
