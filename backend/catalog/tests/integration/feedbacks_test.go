//go:build integration

package integration

import (
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/feedbacks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/suite"
)

func TestFeedbacksSuite(t *testing.T) {
	suite.Run(t, new(DatabaseSuite))
}

func (s *DatabaseSuite) Test_FeedbacksRepository_GetMyCreatedFeedbacks() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))
	s.Require().NoError(s.loadFixtures("feedbacks_list.sql"))

	userID := uuid.MustParse("01b10112-c339-715e-8feb-4751a1ced5ef")
	feedbacks, err := s.feedback.GetMyCreatedFeedbacks(s.ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(feedbacks)
	s.Require().Len(feedbacks, 2)
}

func (s *DatabaseSuite) Test_FeedbacksRepository_DeleteFeedback() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))
	s.Require().NoError(s.loadFixtures("feedbacks_list.sql"))

	id := uuid.MustParse("01a10112-c339-715d-8feb-4751a1ded5ef")
	s.Require().NoError(s.feedback.DeleteFeedback(s.ctx, id))
	s.Require().Error(s.feedback.DeleteFeedback(s.ctx, id))
}

func (s *DatabaseSuite) Test_FeedbacksRepository_CreateFeedback() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))

	productID := uuid.MustParse("01a10112-c339-715d-8feb-4751a1ced5ef")

	e := &domain.Feedback{
		ID:          uuid.NewV7(),
		UserID:      uuid.NewV7(),
		ProductID:   productID,
		Score:       3,
		Description: new("some feedback"),
	}

	a, err := s.feedback.CreateFeedback(s.ctx, e)
	s.Require().NoError(err)

	s.Require().Equal(e.ID, a.ID)
	s.Require().Equal(e.UserID, a.UserID)
	s.Require().Equal(e.ProductID, a.ProductID)
	s.Require().Equal(e.Score, a.Score)
	s.Require().Equal(*e.Description, *a.Description)
	s.Require().NotEmpty(a.CreatedAt)
}

func (s *DatabaseSuite) Test_FeedbacksRepository_GetProductFeedbacks() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))
	s.Require().NoError(s.loadFixtures("feedbacks_list.sql"))

	productID := uuid.MustParse("01a10112-c339-715d-8feb-4751a1ced5ef")

	first, err := s.feedback.GetProductFeedbacks(s.ctx, productID, nil, 2)
	s.Require().NoError(err)
	s.Require().Len(first, 2)
	s.Require().Equal("01a10112-c339-715d-8feb-4751a1ded5ef", first[0].ID.String())
	s.Require().Equal("01a10112-c339-715d-8feb-4751a1ced5ef", first[1].ID.String())
	s.Require().Nil(first[1].Description)

	cursor := first[1].ID
	second, err := s.feedback.GetProductFeedbacks(s.ctx, productID, &cursor, 2)
	s.Require().NoError(err)
	s.Require().Len(second, 1)
	s.Require().Equal("01a10112-b339-715d-8feb-4751a1ced5ef", second[0].ID.String())
}

func (s *DatabaseSuite) Test_CatalogRepository_FeedbacksStats() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))
	s.Require().NoError(s.loadFixtures("feedbacks_list.sql"))

	// 4, 3, 5
	withFeedbacks := uuid.MustParse("01a10112-c339-715d-8feb-4751a1ced5ef")
	item, err := s.catalog.GetItemByID(s.ctx, withFeedbacks)
	s.Require().NoError(err)
	s.Require().Equal(4.0, item.Rating)
	s.Require().Equal(int64(3), item.FeedbacksCount)

	withoutFeedbacks := uuid.MustParse("01a10113-0f6f-70d8-bf84-12447fb5d2aa")
	item, err = s.catalog.GetItemByID(s.ctx, withoutFeedbacks)
	s.Require().NoError(err)
	s.Require().Zero(item.Rating)
	s.Require().Zero(item.FeedbacksCount)

	// lateral не размножает строки: товаров в фикстуре 7, отзывов 4
	page, err := s.catalog.GetItemsByCursorPagination(s.ctx, nil, 100)
	s.Require().NoError(err)
	s.Require().Len(page, 7)

	// у товаров фикстуры нет владельца, картинку меняет только он
	businessID := uuid.NewV7()
	_, err = s.pool.Exec(s.ctx, `UPDATE catalog SET business_id = $1 WHERE product_id = $2`, businessID, withFeedbacks)
	s.Require().NoError(err)

	updated, err := s.catalog.UpdateImageKey(s.ctx, "new_key", withFeedbacks, businessID)
	s.Require().NoError(err)
	s.Require().Equal(int64(3), updated.FeedbacksCount)
}

func (s *DatabaseSuite) Test_FeedbacksRepository_CreateFeedback_ProductMissingOrDeleted() {
	businessID, productID := uuid.NewV7(), uuid.NewV7()
	_, err := s.catalog.Insert(s.ctx, &domain.Product{ID: productID, BusinessID: businessID, Title: "t", Price: 100})
	s.Require().NoError(err)
	s.Require().NoError(s.catalog.DeleteProduct(s.ctx, productID, businessID))

	for name, id := range map[string]uuid.UUID{"deleted": productID, "unknown": uuid.NewV7()} {
		s.T().Run(name, func(t *testing.T) {
			f, err := s.feedback.CreateFeedback(s.ctx, &domain.Feedback{ID: uuid.NewV7(), ProductID: id, UserID: uuid.NewV7(), Score: 3})
			s.Require().ErrorIs(err, feedbacks.ErrNotFound)
			s.Require().Nil(f)
		})
	}
}
