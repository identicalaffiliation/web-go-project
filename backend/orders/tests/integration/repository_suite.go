//go:build integration

package integration

import (
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

type RepositorySuite struct {
	suite.Suite

	pool *pgxpool.Pool
	repo *postgres.Repository
}

func (s *RepositorySuite) SetupSuite() {
	s.pool = startPostgres(s.T())
	s.repo = postgres.NewRepository(s.pool, s.pool)
}

func (s *RepositorySuite) TearDownTest() {
	_, err := s.pool.Exec(s.T().Context(), `TRUNCATE TABLE orders`)
	s.Require().NoError(err)
}

func newOrder(userID uuid.UUID) *domain.Order {
	return &domain.Order{
		ID:          uuid.NewV7(),
		ProductID:   uuid.NewV7(),
		UserID:      userID,
		OperationID: uuid.NewV7(),
		Quantity:    2,
		TotalPrice:  200,
	}
}

func (s *RepositorySuite) createOrder(order *domain.Order) *domain.Order {
	created, err := s.repo.CreateOrder(s.T().Context(), order)
	s.Require().NoError(err)

	return created
}

// setCreatedAt - сдвигает created_at, чтобы проверить сортировку без sleep.
func (s *RepositorySuite) setCreatedAt(id uuid.UUID, at time.Time) {
	_, err := s.pool.Exec(s.T().Context(), `UPDATE orders SET created_at = $1 WHERE id = $2`, at, id)
	s.Require().NoError(err)
}

func (s *RepositorySuite) ordersCount() int {
	var count int
	s.Require().NoError(s.pool.QueryRow(s.T().Context(), `SELECT COUNT(*) FROM orders`).Scan(&count))

	return count
}

func (s *RepositorySuite) orderStatus(id uuid.UUID) (domain.Status, time.Time) {
	var (
		status    domain.Status
		updatedAt time.Time
	)

	err := s.pool.QueryRow(s.T().Context(), `SELECT status, updated_at FROM orders WHERE id = $1`, id).
		Scan(&status, &updatedAt)
	s.Require().NoError(err)

	return status, updatedAt
}
