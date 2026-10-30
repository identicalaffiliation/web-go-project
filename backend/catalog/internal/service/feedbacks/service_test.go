//go:build unit

package feedbacks

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newService(t *testing.T) (*FeedbackService, *mocks.MockFeedbackRepository) {
	repo := mocks.NewMockFeedbackRepository(gomock.NewController(t))
	return NewService(repo, logger.NewNop()), repo
}

func feedbacksN(productID uuid.UUID, n int) []*domain.Feedback {
	out := make([]*domain.Feedback, n)
	for i := range out {
		out[i] = &domain.Feedback{ID: uuid.NewV7(), ProductID: productID, UserID: uuid.NewV7(), Score: 5}
	}

	return out
}

func TestCreateFeedback(t *testing.T) {
	ctx := context.Background()
	productID, userID := uuid.NewV7(), uuid.NewV7()

	validReq := func() *dto.FeedbackCreateRequest {
		return &dto.FeedbackCreateRequest{
			ProductID:   productID.String(),
			UserID:      userID.String(),
			Score:       4,
			Description: new("good"),
		}
	}

	t.Run("ok", func(t *testing.T) {
		s, repo := newService(t)
		now := time.Now()

		repo.EXPECT().CreateFeedback(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, f *domain.Feedback) (*domain.Feedback, error) {
				require.Equal(t, productID, f.ProductID)
				require.Equal(t, userID, f.UserID)
				require.Equal(t, int8(4), f.Score)
				require.NotEqual(t, uuid.Nil(), f.ID)

				created := *f
				created.CreatedAt = now
				return &created, nil
			},
		)

		resp, err := s.CreateFeedback(ctx, validReq())
		require.NoError(t, err)
		require.Equal(t, productID, resp.ProductID)
		require.Equal(t, userID, resp.UserID)
		require.Equal(t, "good", *resp.Description)
		require.Equal(t, now, resp.CreatedAt)
	})

	t.Run("invalid - validation fails before repo", func(t *testing.T) {
		cases := map[string]func(r *dto.FeedbackCreateRequest){
			"score too high":    func(r *dto.FeedbackCreateRequest) { r.Score = 6 },
			"score zero":        func(r *dto.FeedbackCreateRequest) { r.Score = 0 },
			"no user":           func(r *dto.FeedbackCreateRequest) { r.UserID = "" },
			"bad product id":    func(r *dto.FeedbackCreateRequest) { r.ProductID = "bad" },
			"bad user id":       func(r *dto.FeedbackCreateRequest) { r.UserID = "bad" },
			"blank description": func(r *dto.FeedbackCreateRequest) { r.Description = new("  ") },
		}

		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				s, _ := newService(t)
				req := validReq()
				mutate(req)

				_, err := s.CreateFeedback(ctx, req)
				require.Error(t, err)
				require.True(t, errors.Is(err, dto.ErrInvalidData) || errors.Is(err, dto.ErrInvalidJSON), err)
			})
		}
	})

	t.Run("product not found - FK violation", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().CreateFeedback(ctx, gomock.Any()).Return(nil, &pgconn.PgError{Code: "23503"})

		_, err := s.CreateFeedback(ctx, validReq())
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("check constraint violation", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().CreateFeedback(ctx, gomock.Any()).Return(nil, &pgconn.PgError{Code: "23514"})

		_, err := s.CreateFeedback(ctx, validReq())
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("repo error", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().CreateFeedback(ctx, gomock.Any()).Return(nil, errors.New("db down"))

		_, err := s.CreateFeedback(ctx, validReq())
		require.Error(t, err)
		require.NotErrorIs(t, err, dto.ErrInvalidData)
	})
}

func TestGetMyCreatedFeedbacks(t *testing.T) {
	ctx := context.Background()
	userID := uuid.NewV7()

	t.Run("ok", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().GetMyCreatedFeedbacks(ctx, userID).Return(feedbacksN(uuid.NewV7(), 2), nil)

		resp, err := s.GetMyCreatedFeedbacks(ctx, userID.String())
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})

	t.Run("invalid user id", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.GetMyCreatedFeedbacks(ctx, "bad")
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("repo error", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().GetMyCreatedFeedbacks(ctx, userID).Return(nil, errors.New("db down"))

		_, err := s.GetMyCreatedFeedbacks(ctx, userID.String())
		require.Error(t, err)
	})
}

func TestGetProductFeedbacks(t *testing.T) {
	ctx := context.Background()
	productID := uuid.NewV7()

	t.Run("has more - next cursor is last id", func(t *testing.T) {
		s, repo := newService(t)
		items := feedbacksN(productID, 3)
		repo.EXPECT().GetProductFeedbacks(ctx, productID, (*uuid.UUID)(nil), int64(3)).Return(items, nil)

		page, err := s.GetProductFeedbacks(ctx, productID.String(), nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Feedbacks, 2)
		require.NotNil(t, page.NextCursor)
		require.Equal(t, items[1].ID.String(), *page.NextCursor)
	})

	t.Run("last page - no cursor", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().GetProductFeedbacks(ctx, productID, (*uuid.UUID)(nil), int64(3)).Return(feedbacksN(productID, 1), nil)

		page, err := s.GetProductFeedbacks(ctx, productID.String(), nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Feedbacks, 1)
		require.Nil(t, page.NextCursor)
	})

	t.Run("cursor passed to repo", func(t *testing.T) {
		s, repo := newService(t)
		cursor := uuid.NewV7()
		repo.EXPECT().GetProductFeedbacks(ctx, productID, &cursor, int64(21)).Return(nil, nil)

		page, err := s.GetProductFeedbacks(ctx, productID.String(), new(cursor.String()), 20)
		require.NoError(t, err)
		require.Empty(t, page.Feedbacks)
		require.NotNil(t, page.Feedbacks, "empty list must be [] in JSON, not null")
	})

	t.Run("limit out of range - default 20", func(t *testing.T) {
		for _, limit := range []int64{0, -1, 101} {
			s, repo := newService(t)
			repo.EXPECT().GetProductFeedbacks(ctx, productID, (*uuid.UUID)(nil), int64(21)).Return(nil, nil)

			_, err := s.GetProductFeedbacks(ctx, productID.String(), nil, limit)
			require.NoError(t, err)
		}
	})

	t.Run("invalid product id", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.GetProductFeedbacks(ctx, "bad", nil, 20)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.GetProductFeedbacks(ctx, productID.String(), new("bad"), 20)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("repo error", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().GetProductFeedbacks(ctx, productID, (*uuid.UUID)(nil), int64(21)).Return(nil, errors.New("db down"))

		_, err := s.GetProductFeedbacks(ctx, productID.String(), nil, 20)
		require.Error(t, err)
	})
}

func TestDeleteFeedback(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewV7()

	t.Run("ok", func(t *testing.T) {
		s, repo := newService(t)
		repo.EXPECT().DeleteFeedback(ctx, id).Return(nil)
		require.NoError(t, s.DeleteFeedback(ctx, id.String()))
	})

	t.Run("invalid id", func(t *testing.T) {
		s, _ := newService(t)
		require.ErrorIs(t, s.DeleteFeedback(ctx, "bad"), dto.ErrInvalidData)
	})

	t.Run("repo error passes through", func(t *testing.T) {
		s, repo := newService(t)
		notFound := errors.New("not found")
		repo.EXPECT().DeleteFeedback(ctx, id).Return(notFound)
		require.ErrorIs(t, s.DeleteFeedback(ctx, id.String()), notFound)
	})
}
