package feedbacks

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/utils"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"go.uber.org/zap"
)

type FeedbackService struct {
	repo   ports.FeedbackRepository
	logger ports.Logger
}

func NewService(repo ports.FeedbackRepository, logger ports.Logger) *FeedbackService {
	return &FeedbackService{
		repo:   repo,
		logger: logger,
	}
}

func (s *FeedbackService) CreateFeedback(ctx context.Context, req *dto.FeedbackCreateRequest) (*dto.FeedbackResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	f, err := req.ToDomain()
	if err != nil {
		return nil, err
	}

	created, err := s.repo.CreateFeedback(ctx, f)
	if err != nil {
		// FK - товара нет, check - score вне 1..5
		if utils.IsForeignConstraint(err) || utils.IsCheckConstraint(err) {
			return nil, dto.ErrInvalidData
		}

		s.logger.WithError(err).Error("failed to create feedback")
		return nil, err
	}

	return toResponse(created), nil
}

func (s *FeedbackService) GetMyCreatedFeedbacks(ctx context.Context, userID string) ([]*dto.FeedbackResponse, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	feedbacks, err := s.repo.GetMyCreatedFeedbacks(ctx, id)
	if err != nil {
		s.logger.WithError(err).With(zap.String("user id", id.String())).Error("failed to get feedbacks")
		return nil, err
	}

	response := make([]*dto.FeedbackResponse, 0, len(feedbacks))
	for _, v := range feedbacks {
		response = append(response, toResponse(v))
	}

	return response, nil
}

func (s *FeedbackService) GetProductFeedbacks(ctx context.Context, productID string, cursor *string, limit int64) (*dto.FeedbacksPage, error) {
	pID, err := uuid.Parse(productID)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	var c *uuid.UUID
	if cursor != nil && *cursor != "" {
		id, err := uuid.Parse(*cursor)
		if err != nil {
			return nil, dto.ErrInvalidData
		}

		c = &id
	}

	if limit < 1 || limit > 100 {
		limit = 20
	}

	feedbacks, err := s.repo.GetProductFeedbacks(ctx, pID, c, limit+1)
	if err != nil {
		s.logger.WithError(err).With(zap.String("product id", pID.String())).Error("failed to get product feedbacks")
		return nil, err
	}

	hasMore := int64(len(feedbacks)) > limit
	if hasMore {
		feedbacks = feedbacks[:limit]
	}

	var nextCursor *string
	if hasMore && len(feedbacks) > 0 {
		next := feedbacks[len(feedbacks)-1].ID.String()
		nextCursor = &next
	}

	response := make([]*dto.FeedbackResponse, 0, len(feedbacks))
	for _, v := range feedbacks {
		response = append(response, toResponse(v))
	}

	return &dto.FeedbacksPage{Feedbacks: response, NextCursor: nextCursor}, nil
}

func (s *FeedbackService) DeleteFeedback(ctx context.Context, id string) error {
	fID, err := uuid.Parse(id)
	if err != nil {
		return dto.ErrInvalidData
	}

	return s.repo.DeleteFeedback(ctx, fID)
}

func toResponse(f *domain.Feedback) *dto.FeedbackResponse {
	return &dto.FeedbackResponse{
		ID:          f.ID,
		ProductID:   f.ProductID,
		UserID:      f.UserID,
		Score:       f.Score,
		Description: f.Description,
		CreatedAt:   f.CreatedAt,
	}
}
