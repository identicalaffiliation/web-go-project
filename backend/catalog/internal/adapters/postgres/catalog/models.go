package catalog

import (
	"database/sql"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type productModel struct {
	ID          uuid.UUID        `db:"product_id"`
	Title       string           `db:"title"`
	Description sql.Null[string] `db:"description"`
	Price       int64            `db:"price"`
	ImageKey    sql.Null[string] `db:"image_key"`
	CreatedAt   time.Time        `db:"created_at"`
	UpdatedAt   time.Time        `db:"updated_at"`
}

func (m *productModel) toDomain() *domain.Product {
	return &domain.Product{
		ID:          m.ID,
		Title:       m.Title,
		Description: m.Description.V,
		Price:       m.Price,
		ImageKey:    m.ImageKey.V,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
