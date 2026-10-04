//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rest/handlers"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type HandlersSuite struct {
	suite.Suite
	createPort  *mocks.MockAddToCatalogCase
	getItemPort *mocks.MockGetItemCase
	getPagePort *mocks.MockGetPageCase
	e           *echo.Echo
}

func (s *HandlersSuite) Test_AddItemToCatalogHandler() {
	body := `{"title":"test item","price":12000}`

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	s.Require().NoError(mp.WriteField("metadata", body))
	s.Require().NoError(mp.Close())

	request := httptest.NewRequest(http.MethodPost, "/catalog", &buf)
	request.Header.Set(echo.HeaderContentType, mp.FormDataContentType())
	response := httptest.NewRecorder()

	product := &dto.ProductResponse{Product: dto.Product{ProductID: uuid.New(), Title: "test item", Price: 12000}}
	s.createPort.EXPECT().AddProductToCatalog(gomock.Any(), gomock.Any()).Return(product, nil).Times(1)

	s.e.ServeHTTP(response, request)
	s.Require().Equal(http.StatusCreated, response.Code)
	s.Require().Contains(response.Body.String(), "test item")
	s.Require().Contains(response.Body.String(), "12000")
}

func (s *HandlersSuite) Test_AddItemToCatalogHandler_InvalidForm() {
	body := `{"title":"test item","price":12000}`

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	s.Require().NoError(mp.WriteField("invalid", body))
	s.Require().NoError(mp.Close())

	request := httptest.NewRequest(http.MethodPost, "/catalog", &buf)
	request.Header.Set(echo.HeaderContentType, mp.FormDataContentType())
	response := httptest.NewRecorder()

	s.e.ServeHTTP(response, request)
	s.Require().Equal(http.StatusBadRequest, response.Code)
}

func (s *HandlersSuite) Test_AddItemToCatalogHandler_InvalidJSON() {
	body := `.!.`

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	s.Require().NoError(mp.WriteField("metadata", body))
	s.Require().NoError(mp.Close())

	request := httptest.NewRequest(http.MethodPost, "/catalog", &buf)
	request.Header.Set(echo.HeaderContentType, mp.FormDataContentType())
	response := httptest.NewRecorder()

	s.e.ServeHTTP(response, request)
	s.Require().Equal(http.StatusBadRequest, response.Code)
}

func (s *HandlersSuite) Test_GetItemHandler() {

	s.T().Run("success", func(t *testing.T) {
		id := uuid.NewV7()
		request := httptest.NewRequest(http.MethodGet, "/catalog/"+id.String(), nil)
		response := httptest.NewRecorder()

		p := &dto.ProductResponse{
			Product: dto.Product{
				ProductID:    id,
				Title:        "test title",
				Price:        int64(100),
				PresignedURL: new("some url"),
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			},
		}

		s.getItemPort.EXPECT().GetItem(gomock.Any(), gomock.Any()).
			Return(p, nil).
			Times(1)

		s.e.ServeHTTP(response, request)
		s.Require().Equal(http.StatusOK, response.Code)
		s.Require().Contains(response.Body.String(), id.String())
		s.Require().Contains(response.Body.String(), "test title")
		s.Require().Contains(response.Body.String(), "some url")
	})

	s.T().Run("bad request - invalid id param", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/catalog/"+uuid.NewV7().String(), nil)
		response := httptest.NewRecorder()

		s.getItemPort.EXPECT().GetItem(gomock.Any(), gomock.Any()).
			Return(nil, dto.ErrInvalidData).
			Times(1)

		s.e.ServeHTTP(response, request)
		s.Require().Equal(http.StatusBadRequest, response.Code)
	})
}

func (s *HandlersSuite) Test_GetPageHandler() {
	s.T().Run("bad request - invalid limit query param", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/catalog?limit=invalid", nil)
		response := httptest.NewRecorder()

		s.e.ServeHTTP(response, request)
		s.Require().Equal(http.StatusBadRequest, response.Code)
	})

	s.T().Run("success - no pagination, no limit(base = 50)", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/catalog", nil)
		response := httptest.NewRecorder()

		id := uuid.NewV7()

		p := &dto.Page{
			Products: append([]dto.Product{}, dto.Product{
				ProductID: id,
			}),
			NextCursor: nil,
		}

		s.getPagePort.EXPECT().GetItemsPage(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(p, nil).
			Times(1)

		s.e.ServeHTTP(response, request)
		s.Require().Equal(http.StatusOK, response.Code)
		s.Require().Contains(response.Body.String(), id.String())
	})

	s.T().Run("success - limit in query", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/catalog?limit="+strconv.Itoa(3), nil)
		response := httptest.NewRecorder()

		id := uuid.NewV7()
		prelastID := uuid.NewV7()
		if id.Compare(prelastID) == +1 {
			prelastID, id = id, prelastID
		}

		p := &dto.Page{
			Products: []dto.Product{
				{ProductID: id},
				{ProductID: id},
				{ProductID: prelastID},
			},

			NextCursor: new(prelastID.String()),
		}

		s.getPagePort.EXPECT().GetItemsPage(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(p, nil).
			Times(1)

		var pageRespone dto.Page

		s.e.ServeHTTP(response, request)

		s.Require().NoError(json.Unmarshal(response.Body.Bytes(), &pageRespone))
		s.Require().Len(pageRespone.Products, 3)
		s.Require().Equal(pageRespone.Products[len(pageRespone.Products)-1].ProductID, prelastID)
		s.Require().Equal(http.StatusOK, response.Code)
	})

	s.T().Run("success - cursor in query", func(t *testing.T) {
		cursor := uuid.NewV7().String()
		request := httptest.NewRequest(http.MethodGet, "/catalog?cursor="+cursor, nil)
		response := httptest.NewRecorder()

		p := &dto.Page{
			Products: []dto.Product{
				{ProductID: uuid.NewV7()},
				{ProductID: uuid.NewV7()},
				{ProductID: uuid.NewV7()},
			},

			NextCursor: nil,
		}

		s.getPagePort.EXPECT().GetItemsPage(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(p, nil).
			Times(1)

		var pageRespone dto.Page

		s.e.ServeHTTP(response, request)

		s.Require().NoError(json.Unmarshal(response.Body.Bytes(), &pageRespone))
		s.Require().Len(pageRespone.Products, 3)
		s.Require().NotContains(response.Body.String(), cursor)
		s.Require().Equal(http.StatusOK, response.Code)
	})
}

func (s *HandlersSuite) SetupSuite() {
	s.e = echo.New()
}

func (s *HandlersSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	s.createPort = mocks.NewMockAddToCatalogCase(ctrl)
	s.getItemPort = mocks.NewMockGetItemCase(ctrl)
	s.getPagePort = mocks.NewMockGetPageCase(ctrl)

	s.e.POST("/catalog", handlers.AddItemToCatalog(s.createPort))
	s.e.GET("/catalog/:productId", handlers.GetItem(s.getItemPort))
	s.e.GET("/catalog", handlers.GetItemsPage(s.getPagePort))
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}
