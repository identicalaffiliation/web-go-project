package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BusinessProfileCreated — событие Profile «пользователю создан бизнес-профиль».
// Контракт предложен командой и ждёт подтверждения Profile. Обязательны только
// event_id и user_id, лишние поля игнорируются.
type BusinessProfileCreated struct {
	EventID    uuid.UUID `json:"event_id"`
	UserID     uuid.UUID `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ParseBusinessProfileCreated разбирает тело сообщения. Любая проблема с
// содержимым — ErrInvalidEvent: повторная доставка такое сообщение не починит.
// Тело в текст ошибки не попадает, чтобы не утекло в логи.
func ParseBusinessProfileCreated(raw []byte) (BusinessProfileCreated, error) {
	var ev BusinessProfileCreated
	if err := json.Unmarshal(raw, &ev); err != nil {
		// Исходную ошибку json не оборачиваем: в её тексте может быть кусок тела.
		// Исходную ошибку json не оборачиваем: в её тексте может быть кусок тела.
		return BusinessProfileCreated{}, fmt.Errorf("%w: malformed json", ErrInvalidEvent)
	}
	if ev.EventID == uuid.Nil {
		return BusinessProfileCreated{}, fmt.Errorf("%w: event_id is missing", ErrInvalidEvent)
	}
	if ev.UserID == uuid.Nil {
		return BusinessProfileCreated{}, fmt.Errorf("%w: user_id is missing", ErrInvalidEvent)
	}
	return ev, nil
}
