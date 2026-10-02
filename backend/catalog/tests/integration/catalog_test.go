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
	suite.Run(t, new(Suite))
}

func (s *Suite) Test_CatalogRepository_Insert() {
	t := s.T()
	expected := &domain.Product{
		ID:          uuid.New(),
		Title:       "some title",
		Description: "some description",
		Price:       120000,
		ImageKey:    "some key1",
	}

	actual, err := s.catalog.Insert(s.ctx, expected)
	require.NoError(t, err)

	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.Title, actual.Title)
	require.Equal(t, expected.Description, actual.Description)
	require.Equal(t, expected.Price, actual.Price)
	require.Equal(t, expected.ImageKey, actual.ImageKey)
	require.NotEmpty(t, actual.CreatedAt)
	require.NotEmpty(t, actual.UpdatedAt)
}

func (s *Suite) Test_CatalogRepository_Insert_CheckConstraintsFail() {

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

func (s *Suite) Test_CatalogRepository_Insert_EmptyFields() {

	s.T().Run("empty field - description", func(t *testing.T) {
		expected := &domain.Product{
			ID:       uuid.New(),
			Title:    "test title",
			Price:    123,
			ImageKey: "some key4",
		}

		actual, err := s.catalog.Insert(s.ctx, expected)
		s.Require().NoError(err)
		s.Require().NotNil(actual)
		s.Require().Empty(actual.Description)
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
