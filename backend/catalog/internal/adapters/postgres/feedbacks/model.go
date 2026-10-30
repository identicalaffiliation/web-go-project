package feedbacks

import (
	"database/sql"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type feedbackModel struct {
	ID          uuid.UUID        `db:"id"`
	ProductID   uuid.UUID        `db:"product_id"`
	UserID      uuid.UUID        `db:"user_id"`
	Score       int8             `db:"score"`
	Description sql.Null[string] `db:"description"`
	CreatedAt   time.Time        `db:"created_at"`
}

func (m *feedbackModel) toDomain() *domain.Feedback {
	var description *string
	if m.Description.Valid {
		description = new(m.Description.V)
	}

	return &domain.Feedback{
		ID:          m.ID,
		ProductID:   m.ProductID,
		UserID:      m.UserID,
		Score:       m.Score,
		Description: description,
		CreatedAt:   m.CreatedAt,
	}
}
