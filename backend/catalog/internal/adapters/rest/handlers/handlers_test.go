//go:build unit

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// serve - прогоняет запрос через echo-роутер, чтобы path-параметры резолвились как в проде.
func serve(method, route string, h echo.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	e := echo.New()
	e.Add(method, route, h)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func jsonRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func TestAddItemToCatalog(t *testing.T) {
	const route = "/catalog"
	body := `{"title":"t","price":100,"quantity":5}`

	t.Run("201", func(t *testing.T) {
		app := mocks.NewMockAddToCatalogCase(gomock.NewController(t))
		id := uuid.NewV7()
		app.EXPECT().AddProductToCatalog(gomock.Any(), &dto.CreateProductRequest{Title: "t", Price: 100, Quantity: 5}).
			Return(&dto.ProductResponse{Product: dto.Product{ProductID: id}}, nil)

		rec := serve(http.MethodPost, route, AddItemToCatalog(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusCreated, rec.Code)

		var resp dto.ProductResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, id, resp.Product.ProductID)
	})

	t.Run("400 - broken json", func(t *testing.T) {
		app := mocks.NewMockAddToCatalogCase(gomock.NewController(t))
		rec := serve(http.MethodPost, route, AddItemToCatalog(app), jsonRequest(http.MethodPost, route, `{`))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("400 - validation", func(t *testing.T) {
		app := mocks.NewMockAddToCatalogCase(gomock.NewController(t))
		rec := serve(http.MethodPost, route, AddItemToCatalog(app), jsonRequest(http.MethodPost, route, `{"title":"t","price":0,"quantity":5}`))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("400 - invalid data from service", func(t *testing.T) {
		app := mocks.NewMockAddToCatalogCase(gomock.NewController(t))
		app.EXPECT().AddProductToCatalog(gomock.Any(), gomock.Any()).Return(nil, dto.ErrInvalidData)

		rec := serve(http.MethodPost, route, AddItemToCatalog(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockAddToCatalogCase(gomock.NewController(t))
		app.EXPECT().AddProductToCatalog(gomock.Any(), gomock.Any()).Return(nil, catalog.ErrInternal)

		rec := serve(http.MethodPost, route, AddItemToCatalog(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestGetItem(t *testing.T) {
	const route = "/catalog/:productId"
	id := uuid.NewV7()

	t.Run("200", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(gomock.Any(), id.String()).
			Return(&dto.ProductResponse{Product: dto.Product{ProductID: id, Rating: 4.5, FeedbacksCount: 2}}, nil)

		rec := serve(http.MethodGet, route, GetItem(app), httptest.NewRequest(http.MethodGet, "/catalog/"+id.String(), nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"rating":4.5`)
		require.Contains(t, rec.Body.String(), `"feedbacksCount":2`)
	})

	t.Run("400", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(gomock.Any(), "bad").Return(nil, dto.ErrInvalidData)

		rec := serve(http.MethodGet, route, GetItem(app), httptest.NewRequest(http.MethodGet, "/catalog/bad", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(gomock.Any(), id.String()).Return(nil, catalog.ErrInternal)

		rec := serve(http.MethodGet, route, GetItem(app), httptest.NewRequest(http.MethodGet, "/catalog/"+id.String(), nil))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestGetItemsPage(t *testing.T) {
	const route = "/catalog"

	t.Run("200 - defaults", func(t *testing.T) {
		app := mocks.NewMockGetPageCase(gomock.NewController(t))
		app.EXPECT().GetItemsPage(gomock.Any(), (*string)(nil), int64(50)).Return(&dto.Page{}, nil)

		rec := serve(http.MethodGet, route, GetItemsPage(app), httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("200 - limit and cursor passed", func(t *testing.T) {
		app := mocks.NewMockGetPageCase(gomock.NewController(t))
		app.EXPECT().GetItemsPage(gomock.Any(), new("abc"), int64(10)).Return(&dto.Page{}, nil)

		rec := serve(http.MethodGet, route, GetItemsPage(app), httptest.NewRequest(http.MethodGet, route+"?limit=10&cursor=abc", nil))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("400 - bad limit", func(t *testing.T) {
		app := mocks.NewMockGetPageCase(gomock.NewController(t))
		rec := serve(http.MethodGet, route, GetItemsPage(app), httptest.NewRequest(http.MethodGet, route+"?limit=ten", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockGetPageCase(gomock.NewController(t))
		app.EXPECT().GetItemsPage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, catalog.ErrInternal)

		rec := serve(http.MethodGet, route, GetItemsPage(app), httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestUploadImage(t *testing.T) {
	const route = "/catalog/:productId/image"
	id := uuid.NewV7()
	target := "/catalog/" + id.String() + "/image?format=png"

	cases := []struct {
		name string
		err  error
		code int
	}{
		{"200", nil, http.StatusOK},
		{"400", dto.ErrInvalidData, http.StatusBadRequest},
		{"404", catalog.ErrNotFound, http.StatusNotFound},
		{"500", catalog.ErrInternal, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := mocks.NewMockUploadImageCase(gomock.NewController(t))
			var resp *dto.ProductResponse
			if tc.err == nil {
				resp = &dto.ProductResponse{Product: dto.Product{ProductID: id, PresignedURL: new("http://upload")}}
			}
			app.EXPECT().UploadImage(gomock.Any(), id.String(), dto.PNG).Return(resp, tc.err)

			rec := serve(http.MethodPost, route, UploadImage(app), httptest.NewRequest(http.MethodPost, target, nil))
			require.Equal(t, tc.code, rec.Code)
		})
	}
}

func TestLikeProduct(t *testing.T) {
	const route = "/catalog/like"
	id := uuid.NewV7().String()
	body := `{"id":"` + id + `","action":"add"}`

	t.Run("201", func(t *testing.T) {
		app := mocks.NewMockLikeItem(gomock.NewController(t))
		app.EXPECT().LikeItem(gomock.Any(), id, dto.LikeAdd).Return(nil)

		rec := serve(http.MethodPost, route, LikeProduct(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("400 - broken json", func(t *testing.T) {
		app := mocks.NewMockLikeItem(gomock.NewController(t))
		rec := serve(http.MethodPost, route, LikeProduct(app), jsonRequest(http.MethodPost, route, `{`))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("400 - invalid from service", func(t *testing.T) {
		app := mocks.NewMockLikeItem(gomock.NewController(t))
		app.EXPECT().LikeItem(gomock.Any(), id, dto.LikeAdd).Return(catalog.ErrInvalid)

		rec := serve(http.MethodPost, route, LikeProduct(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockLikeItem(gomock.NewController(t))
		app.EXPECT().LikeItem(gomock.Any(), id, dto.LikeAdd).Return(errors.New("db down"))

		rec := serve(http.MethodPost, route, LikeProduct(app), jsonRequest(http.MethodPost, route, body))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestCreateFeedback(t *testing.T) {
	const route = "/catalog/feedbacks"
	productID, userID := uuid.NewV7(), uuid.NewV7()
	body := `{"productId":"` + productID.String() + `","score":5,"description":"good"}`

	withUser := func() *http.Request {
		req := jsonRequest(http.MethodPost, route, body)
		req.Header.Set(userIDHeader, userID.String())
		return req
	}

	t.Run("201 - user id taken from header", func(t *testing.T) {
		app := mocks.NewMockCreateFeedbackCase(gomock.NewController(t))
		app.EXPECT().CreateFeedback(gomock.Any(), &dto.FeedbackCreateRequest{
			ProductID:   productID.String(),
			UserID:      userID.String(),
			Score:       5,
			Description: new("good"),
		}).Return(&dto.FeedbackResponse{ID: uuid.NewV7(), ProductID: productID, UserID: userID, Score: 5}, nil)

		rec := serve(http.MethodPost, route, CreateFeedback(app), withUser())
		require.Equal(t, http.StatusCreated, rec.Code)

		var resp dto.FeedbackResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, userID, resp.UserID)
	})

	t.Run("user id in body is ignored", func(t *testing.T) {
		app := mocks.NewMockCreateFeedbackCase(gomock.NewController(t))
		app.EXPECT().CreateFeedback(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ any, req *dto.FeedbackCreateRequest) (*dto.FeedbackResponse, error) {
				require.Equal(t, userID.String(), req.UserID)
				return &dto.FeedbackResponse{}, nil
			},
		)

		req := jsonRequest(http.MethodPost, route, `{"productId":"`+productID.String()+`","score":5,"userId":"`+uuid.NewV7().String()+`"}`)
		req.Header.Set(userIDHeader, userID.String())

		rec := serve(http.MethodPost, route, CreateFeedback(app), req)
		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("400 - broken json", func(t *testing.T) {
		app := mocks.NewMockCreateFeedbackCase(gomock.NewController(t))
		rec := serve(http.MethodPost, route, CreateFeedback(app), jsonRequest(http.MethodPost, route, `{`))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	for _, err := range []error{dto.ErrInvalidJSON, dto.ErrInvalidData} {
		t.Run("400 - "+err.Error(), func(t *testing.T) {
			app := mocks.NewMockCreateFeedbackCase(gomock.NewController(t))
			app.EXPECT().CreateFeedback(gomock.Any(), gomock.Any()).Return(nil, err)

			rec := serve(http.MethodPost, route, CreateFeedback(app), withUser())
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockCreateFeedbackCase(gomock.NewController(t))
		app.EXPECT().CreateFeedback(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

		rec := serve(http.MethodPost, route, CreateFeedback(app), withUser())
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestGetProductFeedbacks(t *testing.T) {
	const route = "/catalog/:productId/feedbacks"
	productID := uuid.NewV7().String()
	target := "/catalog/" + productID + "/feedbacks"

	t.Run("200 - defaults", func(t *testing.T) {
		app := mocks.NewMockGetProductFeedbacksCase(gomock.NewController(t))
		app.EXPECT().GetProductFeedbacks(gomock.Any(), productID, (*string)(nil), int64(20)).
			Return(&dto.FeedbacksPage{Feedbacks: []*dto.FeedbackResponse{}}, nil)

		rec := serve(http.MethodGet, route, GetProductFeedbacks(app), httptest.NewRequest(http.MethodGet, target, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"feedbacks":[]}`, rec.Body.String())
	})

	t.Run("200 - limit and cursor passed", func(t *testing.T) {
		app := mocks.NewMockGetProductFeedbacksCase(gomock.NewController(t))
		app.EXPECT().GetProductFeedbacks(gomock.Any(), productID, new("c1"), int64(5)).
			Return(&dto.FeedbacksPage{Feedbacks: []*dto.FeedbackResponse{}, NextCursor: new("c2")}, nil)

		rec := serve(http.MethodGet, route, GetProductFeedbacks(app), httptest.NewRequest(http.MethodGet, target+"?limit=5&cursor=c1", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"nextCursor":"c2"`)
	})

	t.Run("400 - bad limit", func(t *testing.T) {
		app := mocks.NewMockGetProductFeedbacksCase(gomock.NewController(t))
		rec := serve(http.MethodGet, route, GetProductFeedbacks(app), httptest.NewRequest(http.MethodGet, target+"?limit=x", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("400 - invalid from service", func(t *testing.T) {
		app := mocks.NewMockGetProductFeedbacksCase(gomock.NewController(t))
		app.EXPECT().GetProductFeedbacks(gomock.Any(), productID, gomock.Any(), gomock.Any()).Return(nil, dto.ErrInvalidData)

		rec := serve(http.MethodGet, route, GetProductFeedbacks(app), httptest.NewRequest(http.MethodGet, target, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("500", func(t *testing.T) {
		app := mocks.NewMockGetProductFeedbacksCase(gomock.NewController(t))
		app.EXPECT().GetProductFeedbacks(gomock.Any(), productID, gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

		rec := serve(http.MethodGet, route, GetProductFeedbacks(app), httptest.NewRequest(http.MethodGet, target, nil))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
