package feedbacks

import (
	"context"
	"errors"
	"uuid"

	feedbacksRepo "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/feedbacks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/utils"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"go.uber.org/zap"
)

type FeedbackService struct {
	repo   ports.FeedbackRepository
	cache  ports.Cache
	logger ports.Logger
}

func NewService(repo ports.FeedbackRepository, cache ports.Cache, logger ports.Logger) *FeedbackService {
	return &FeedbackService{
		repo:   repo,
		cache:  cache,
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
		// not found / FK - товара нет или он удалён, check - score вне 1..5
		if errors.Is(err, feedbacksRepo.ErrNotFound) || utils.IsForeignConstraint(err) || utils.IsCheckConstraint(err) {
			return nil, dto.ErrInvalidData
		}

		s.logger.WithError(err).Error("failed to create feedback")
		return nil, err
	}

	// рейтинг и число отзывов карточки считаются из отзывов - убираем её из кэша, иначе
	// новый отзыв не виден до TTL. Отзыв уже сохранён, ошибка кэша его не откатывает
	if err := s.cache.Delete(ctx, created.ProductID.String()); err != nil {
		s.logger.WithError(err).Error(
			"failed to delete product from cache",
			zap.String("product id", created.ProductID.String()),
		)
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

	c, err := parseCursor(cursor)
	if err != nil {
		return nil, err
	}

	limit = pageLimit(limit)
	feedbacks, err := s.repo.GetProductFeedbacks(ctx, pID, c, limit+1)
	if err != nil {
		s.logger.WithError(err).With(zap.String("product id", pID.String())).Error("failed to get product feedbacks")
		return nil, err
	}

	return toPage(feedbacks, limit), nil
}

// GetBusinessFeedbacks - отзывы на товары бизнеса (владелец товара - businessID), новые первыми.
func (s *FeedbackService) GetBusinessFeedbacks(ctx context.Context, businessID string, cursor *string, limit int64) (*dto.FeedbacksPage, error) {
	bID, err := uuid.Parse(businessID)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	c, err := parseCursor(cursor)
	if err != nil {
		return nil, err
	}

	limit = pageLimit(limit)
	feedbacks, err := s.repo.GetBusinessFeedbacks(ctx, bID, c, limit+1)
	if err != nil {
		s.logger.WithError(err).With(zap.String("business id", bID.String())).Error("failed to get business feedbacks")
		return nil, err
	}

	return toPage(feedbacks, limit), nil
}

func (s *FeedbackService) DeleteFeedback(ctx context.Context, id string) error {
	fID, err := uuid.Parse(id)
	if err != nil {
		return dto.ErrInvalidData
	}

	return s.repo.DeleteFeedback(ctx, fID)
}

func parseCursor(cursor *string) (*uuid.UUID, error) {
	if cursor == nil || *cursor == "" {
		return nil, nil
	}

	id, err := uuid.Parse(*cursor)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	return &id, nil
}

func pageLimit(limit int64) int64 {
	if limit < 1 || limit > 100 {
		return 20
	}

	return limit
}

// toPage - feedbacks запрошены с limit+1: лишний отзыв значит, что есть следующая страница.
func toPage(feedbacks []*domain.Feedback, limit int64) *dto.FeedbacksPage {
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

	return &dto.FeedbacksPage{Feedbacks: response, NextCursor: nextCursor}
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
