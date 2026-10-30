//go:build unit

package feedbacks

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	feedbacksv1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/feedbacks/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/feedbacks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetMyCreatedFeedbacks(t *testing.T) {
	ctx := context.Background()
	userID := uuid.NewV7()

	t.Run("ok - mapped to proto", func(t *testing.T) {
		app := mocks.NewMockFeedbackService(gomock.NewController(t))
		createdAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		f := &dto.FeedbackResponse{
			ID:          uuid.NewV7(),
			ProductID:   uuid.NewV7(),
			UserID:      userID,
			Score:       4,
			Description: new("good"),
			CreatedAt:   createdAt,
		}
		app.EXPECT().GetMyCreatedFeedbacks(ctx, userID.String()).
			Return([]*dto.FeedbackResponse{f, {ID: uuid.NewV7(), UserID: userID, Score: 1}}, nil)

		s := &GRPCServer{app: app}
		resp, err := s.GetMyCreatedFeedbacks(ctx, &feedbacksv1.GetMyFeedbacksRequest{UserId: userID.String()})
		require.NoError(t, err)
		require.Len(t, resp.GetFeedbacks(), 2)

		got := resp.GetFeedbacks()[0]
		require.Equal(t, f.ID.String(), got.GetId())
		require.Equal(t, f.ProductID.String(), got.GetProductId())
		require.Equal(t, userID.String(), got.GetUserId())
		require.Equal(t, int32(4), got.GetScore())
		require.Equal(t, "good", got.GetDescription())
		require.Equal(t, createdAt, got.GetCreatedAt().AsTime())

		require.Nil(t, resp.GetFeedbacks()[1].Description, "no description - field absent")
	})

	t.Run("invalid argument", func(t *testing.T) {
		app := mocks.NewMockFeedbackService(gomock.NewController(t))
		app.EXPECT().GetMyCreatedFeedbacks(ctx, "bad").Return(nil, dto.ErrInvalidData)

		s := &GRPCServer{app: app}
		_, err := s.GetMyCreatedFeedbacks(ctx, &feedbacksv1.GetMyFeedbacksRequest{UserId: "bad"})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("internal", func(t *testing.T) {
		app := mocks.NewMockFeedbackService(gomock.NewController(t))
		app.EXPECT().GetMyCreatedFeedbacks(ctx, userID.String()).Return(nil, errors.New("db down"))

		s := &GRPCServer{app: app}
		_, err := s.GetMyCreatedFeedbacks(ctx, &feedbacksv1.GetMyFeedbacksRequest{UserId: userID.String()})
		require.Equal(t, codes.Internal, status.Code(err))
		require.NotContains(t, err.Error(), "db down", "internal error details must not leak")
	})
}

func TestDeleteFeedback(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewV7().String()

	cases := []struct {
		name string
		err  error
		code codes.Code
	}{
		{"ok", nil, codes.OK},
		{"invalid argument", dto.ErrInvalidData, codes.InvalidArgument},
		{"not found", feedbacks.ErrNotFound, codes.NotFound},
		{"internal", errors.New("db down"), codes.Internal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := mocks.NewMockFeedbackService(gomock.NewController(t))
			app.EXPECT().DeleteFeedback(ctx, id).Return(tc.err)

			s := &GRPCServer{app: app}
			resp, err := s.DeleteFeedback(ctx, &feedbacksv1.DeleteFeedbackRequest{Id: id})
			require.Equal(t, tc.code, status.Code(err))
			if tc.err == nil {
				require.NotNil(t, resp)
			}
		})
	}
}
