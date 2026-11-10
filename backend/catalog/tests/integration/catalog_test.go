//go:build integration

package integration

import (
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestCatalogSuite(t *testing.T) {
	suite.Run(t, new(DatabaseSuite))
}

func (s *DatabaseSuite) Test_CatalogRepository_Insert() {
	t := s.T()
	expected := &domain.Product{
		ID:          uuid.New(),
		BusinessID:  uuid.NewV7(),
		Title:       "some title",
		Description: "some description",
		Price:       120000,
	}

	actual, err := s.catalog.Insert(s.ctx, expected)
	require.NoError(t, err)

	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.BusinessID, actual.BusinessID)
	require.Equal(t, expected.Title, actual.Title)
	require.Equal(t, expected.Description, actual.Description)
	require.Equal(t, expected.Price, actual.Price)
	require.Empty(t, actual.ImageKey)
	require.NotEmpty(t, actual.CreatedAt)
	require.NotEmpty(t, actual.UpdatedAt)
}

func (s *DatabaseSuite) Test_CatalogRepository_Insert_CheckConstraintsFail() {

	s.T().Run("invalid - negative price field", func(t *testing.T) {
		expected := &domain.Product{
			ID:          uuid.New(),
			Title:       "some title",
			Description: "some description",
			Price:       -100,
			ImageKey:    "some key2",
		}

		actual, err := s.catalog.Insert(s.ctx, expected)
		s.Require().Nil(actual)
		s.Require().Error(err)
		s.Require().ErrorIs(err, catalog.ErrInvalidData)
	})

	s.T().Run("invalid - empty title field", func(t *testing.T) {
		expected := &domain.Product{
			ID:          uuid.New(),
			Title:       "",
			Description: "some description",
			Price:       1,
			ImageKey:    "some key3",
		}

		actual, err := s.catalog.Insert(s.ctx, expected)
		s.Require().Nil(actual)
		s.Require().Error(err)
		s.Require().ErrorIs(err, catalog.ErrInvalidData)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_Insert_EmptyFields() {

	s.T().Run("empty field - description", func(t *testing.T) {
		expected := &domain.Product{
			ID:    uuid.New(),
			Title: "test title",
			Price: 123,
		}

		actual, err := s.catalog.Insert(s.ctx, expected)
		s.Require().NoError(err)
		s.Require().NotNil(actual)
		s.Require().Empty(actual.Description)
		s.Require().Empty(actual.ImageKey)
	})

	s.T().Run("empty field - image key", func(t *testing.T) {
		expected := &domain.Product{
			ID:          uuid.New(),
			Title:       "test title",
			Description: "test desc",
			Price:       123,
		}

		actual, err := s.catalog.Insert(s.ctx, expected)
		s.Require().NoError(err)
		s.Require().NotNil(actual)
		s.Require().Empty(actual.ImageKey)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_GetItemByID() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))

	s.T().Run("valid case", func(t *testing.T) {
		actual, err := s.catalog.GetItemByID(s.ctx, uuid.MustParse("01a10112-c339-715d-8feb-4751a1ced5ef"))
		s.Require().NoError(err)
		s.Require().NotNil(actual)
		s.Require().Equal("test title1", actual.Title)
		s.Require().Equal("test desc1", actual.Description)
		s.Require().Equal(int64(100), actual.Price)
		s.Require().Equal("some_key1", actual.ImageKey)
		s.Require().Equal(uuid.Nil(), actual.BusinessID, "fixture products have no owner (NULL)")
		s.Require().NotEmpty(actual.CreatedAt)
		s.Require().NotEmpty(actual.UpdatedAt)
	})

	s.T().Run("error - not found case", func(t *testing.T) {
		actual, err := s.catalog.GetItemByID(s.ctx, uuid.NewV7())
		s.Require().Error(err)
		s.Require().Nil(actual)
		s.Require().ErrorIs(err, catalog.ErrNotFound)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_GetItemsByCursorPagination() {
	s.Require().NoError(s.loadFixtures("items_list.sql"))

	s.T().Run("no cursor case", func(t *testing.T) {
		items, err := s.catalog.GetItemsByCursorPagination(s.ctx, nil, 2)
		s.Require().NoError(err)
		s.Require().NotNil(items)
		s.Require().Len(items, 2)
	})

	s.T().Run("cursor case", func(t *testing.T) {
		items, err := s.catalog.GetItemsByCursorPagination(s.ctx, new("01a10113-991f-736b-b88b-f1ef00dd0933"), 3)
		s.Require().NoError(err)
		s.Require().NotNil(items)
		s.Require().Len(items, 3)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_GetItemsByBusinessID() {
	// товары без владельца (NULL) в выдачу бизнеса не попадают
	s.Require().NoError(s.loadFixtures("items_list.sql"))

	businessID, otherID := uuid.NewV7(), uuid.NewV7()
	// id по возрастанию: выдача идёт по product_id DESC
	own := []uuid.UUID{
		uuid.MustParse("01a20000-0000-7000-8000-000000000001"),
		uuid.MustParse("01a20000-0000-7000-8000-000000000002"),
		uuid.MustParse("01a20000-0000-7000-8000-000000000003"),
	}
	for _, id := range own {
		_, err := s.catalog.Insert(s.ctx, &domain.Product{ID: id, BusinessID: businessID, Title: "own", Price: 100})
		s.Require().NoError(err)
	}

	_, err := s.catalog.Insert(s.ctx, &domain.Product{
		ID:         uuid.MustParse("01a20000-0000-7000-8000-000000000004"),
		BusinessID: otherID,
		Title:      "other",
		Price:      100,
	})
	s.Require().NoError(err)

	s.T().Run("only owner products, newest first", func(t *testing.T) {
		items, err := s.catalog.GetItemsByBusinessID(s.ctx, businessID, nil, 10)
		s.Require().NoError(err)
		s.Require().Len(items, 3)

		for i, item := range items {
			s.Require().Equal(own[len(own)-1-i], item.ID)
			s.Require().Equal(businessID, item.BusinessID)
		}
	})

	s.T().Run("cursor and limit", func(t *testing.T) {
		items, err := s.catalog.GetItemsByBusinessID(s.ctx, businessID, new(own[2].String()), 1)
		s.Require().NoError(err)
		s.Require().Len(items, 1)
		s.Require().Equal(own[1], items[0].ID)
	})

	s.T().Run("unknown business - empty", func(t *testing.T) {
		items, err := s.catalog.GetItemsByBusinessID(s.ctx, uuid.NewV7(), nil, 10)
		s.Require().NoError(err)
		s.Require().Empty(items)
	})

	s.T().Run("invalid cursor", func(t *testing.T) {
		items, err := s.catalog.GetItemsByBusinessID(s.ctx, businessID, new("bad"), 10)
		s.Require().ErrorIs(err, catalog.ErrInvalidData)
		s.Require().Nil(items)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_UpdateProduct() {
	businessID := uuid.NewV7()
	id := uuid.NewV7()
	_, err := s.catalog.Insert(s.ctx, &domain.Product{ID: id, BusinessID: businessID, Title: "old", Description: "old desc", Price: 100})
	s.Require().NoError(err)

	s.T().Run("only passed fields change", func(t *testing.T) {
		actual, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{
			ProductID:  id,
			BusinessID: businessID,
			Price:      new(int64(250)),
		})
		s.Require().NoError(err)
		s.Require().Equal(id, actual.ID)
		s.Require().Equal(businessID, actual.BusinessID)
		s.Require().Equal(int64(250), actual.Price)
		s.Require().Equal("old", actual.Title)
		s.Require().Equal("old desc", actual.Description)

		actual, err = s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{
			ProductID:   id,
			BusinessID:  businessID,
			Title:       new("new"),
			Description: new("new desc"),
		})
		s.Require().NoError(err)
		s.Require().Equal("new", actual.Title)
		s.Require().Equal("new desc", actual.Description)
		s.Require().Equal(int64(250), actual.Price)

		stored, err := s.catalog.GetItemByID(s.ctx, id)
		s.Require().NoError(err)
		s.Require().Equal("new", stored.Title)
		s.Require().Equal(int64(250), stored.Price)
	})

	s.T().Run("feedback stats in result", func(t *testing.T) {
		_, err := s.feedback.CreateFeedback(s.ctx, &domain.Feedback{ID: uuid.NewV7(), ProductID: id, UserID: uuid.NewV7(), Score: 4})
		s.Require().NoError(err)

		actual, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: id, BusinessID: businessID, Price: new(int64(300))})
		s.Require().NoError(err)
		s.Require().Equal(4.0, actual.Rating)
		s.Require().Equal(int64(1), actual.FeedbacksCount)
	})

	s.T().Run("other business - not found, product untouched", func(t *testing.T) {
		actual, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: id, BusinessID: uuid.NewV7(), Price: new(int64(1))})
		s.Require().ErrorIs(err, catalog.ErrNotFound)
		s.Require().Nil(actual)

		stored, err := s.catalog.GetItemByID(s.ctx, id)
		s.Require().NoError(err)
		s.Require().Equal(int64(300), stored.Price)
	})

	s.T().Run("unknown product - not found", func(t *testing.T) {
		_, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: uuid.NewV7(), BusinessID: businessID, Price: new(int64(1))})
		s.Require().ErrorIs(err, catalog.ErrNotFound)
	})

	s.T().Run("check constraint - invalid data", func(t *testing.T) {
		_, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: id, BusinessID: businessID, Price: new(int64(0))})
		s.Require().ErrorIs(err, catalog.ErrInvalidData)

		_, err = s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: id, BusinessID: businessID, Title: new("")})
		s.Require().ErrorIs(err, catalog.ErrInvalidData)
	})

	s.T().Run("deleted product - not found", func(t *testing.T) {
		s.Require().NoError(s.catalog.DeleteProduct(s.ctx, id, businessID))

		_, err := s.catalog.UpdateProduct(s.ctx, &domain.ProductPatch{ProductID: id, BusinessID: businessID, Price: new(int64(1))})
		s.Require().ErrorIs(err, catalog.ErrNotFound)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_DeleteProduct() {
	businessID := uuid.NewV7()
	// id по возрастанию: выдача идёт по product_id DESC
	kept := uuid.MustParse("01a30000-0000-7000-8000-000000000001")
	deleted := uuid.MustParse("01a30000-0000-7000-8000-000000000002")
	for _, id := range []uuid.UUID{kept, deleted} {
		_, err := s.catalog.Insert(s.ctx, &domain.Product{ID: id, BusinessID: businessID, Title: "own", Price: 100})
		s.Require().NoError(err)
	}

	_, err := s.feedback.CreateFeedback(s.ctx, &domain.Feedback{ID: uuid.NewV7(), ProductID: deleted, UserID: uuid.NewV7(), Score: 5})
	s.Require().NoError(err)

	s.T().Run("other business - not found, product kept", func(t *testing.T) {
		s.Require().ErrorIs(s.catalog.DeleteProduct(s.ctx, deleted, uuid.NewV7()), catalog.ErrNotFound)

		_, err := s.catalog.GetItemByID(s.ctx, deleted)
		s.Require().NoError(err)
	})

	s.T().Run("owner deletes product with feedbacks", func(t *testing.T) {
		s.Require().NoError(s.catalog.DeleteProduct(s.ctx, deleted, businessID))
	})

	s.T().Run("deleted product hidden everywhere", func(t *testing.T) {
		_, err := s.catalog.GetItemByID(s.ctx, deleted)
		s.Require().ErrorIs(err, catalog.ErrNotFound)

		items, err := s.catalog.GetItemsByBusinessID(s.ctx, businessID, nil, 10)
		s.Require().NoError(err)
		s.Require().Len(items, 1)
		s.Require().Equal(kept, items[0].ID)

		items, err = s.catalog.GetItemsByCursorPagination(s.ctx, nil, 10)
		s.Require().NoError(err)
		s.Require().Len(items, 1)
		s.Require().Equal(kept, items[0].ID)

		_, err = s.catalog.UpdateImageKey(s.ctx, "image/deleted.png", deleted, businessID)
		s.Require().ErrorIs(err, catalog.ErrNotFound)
	})

	s.T().Run("second delete - not found", func(t *testing.T) {
		s.Require().ErrorIs(s.catalog.DeleteProduct(s.ctx, deleted, businessID), catalog.ErrNotFound)
	})

	s.T().Run("unknown product - not found", func(t *testing.T) {
		s.Require().ErrorIs(s.catalog.DeleteProduct(s.ctx, uuid.NewV7(), businessID), catalog.ErrNotFound)
	})
}

func (s *DatabaseSuite) Test_CatalogRepository_UpdateImageKey() {
	businessID, id := uuid.NewV7(), uuid.NewV7()
	_, err := s.catalog.Insert(s.ctx, &domain.Product{ID: id, BusinessID: businessID, Title: "t", Price: 100})
	s.Require().NoError(err)

	s.T().Run("owner - key updated", func(t *testing.T) {
		actual, err := s.catalog.UpdateImageKey(s.ctx, "image/own.png", id, businessID)
		s.Require().NoError(err)
		s.Require().Equal("image/own.png", actual.ImageKey)
	})

	s.T().Run("other business - not found, key untouched", func(t *testing.T) {
		actual, err := s.catalog.UpdateImageKey(s.ctx, "image/other.png", id, uuid.NewV7())
		s.Require().ErrorIs(err, catalog.ErrNotFound)
		s.Require().Nil(actual)

		stored, err := s.catalog.GetItemByID(s.ctx, id)
		s.Require().NoError(err)
		s.Require().Equal("image/own.png", stored.ImageKey)
	})

	s.T().Run("product without owner - not found", func(t *testing.T) {
		s.Require().NoError(s.loadFixtures("items_list.sql"))

		_, err := s.catalog.UpdateImageKey(s.ctx, "image/legacy.png", uuid.MustParse("01a10112-c339-715d-8feb-4751a1ced5ef"), businessID)
		s.Require().ErrorIs(err, catalog.ErrNotFound)
	})
}
