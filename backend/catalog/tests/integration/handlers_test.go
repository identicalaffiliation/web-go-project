//go:build integration

package integration

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
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
	createPort *mocks.MockAddToCatalogCase
	e          *echo.Echo
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

func (s *HandlersSuite) SetupSuite() {
	s.e = echo.New()
}

func (s *HandlersSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	s.createPort = mocks.NewMockAddToCatalogCase(ctrl)

	s.e.POST("/catalog", handlers.AddItemToCatalog(s.createPort))
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}
