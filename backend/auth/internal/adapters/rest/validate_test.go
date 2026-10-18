package rest_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func callValidate(v *mocks.MockAccessTokenVerifier, authHeader string) *httptest.ResponseRecorder {
	e := echo.New()
	e.GET("/validate", rest.ValidateHandler(v, logging.NewNop()))

	req := httptest.NewRequest(http.MethodGet, "/validate", http.NoBody)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestValidateHandler_Rejects(t *testing.T) {
	tests := []struct {
		name   string
		header string
		verr   error // не nil — верификатор должен быть вызван и вернуть эту ошибку
	}{
		{name: "no header"},
		{name: "wrong scheme", header: "Basic abc"},
		{name: "scheme without token", header: "Bearer"},
		{name: "verifier error", header: "Bearer tok", verr: errors.New("bad signature")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Мок без ожиданий валит тест при любом вызове: так проверяем,
			// что на кривом заголовке верификатор даже не дёргается.
			v := mocks.NewMockAccessTokenVerifier(gomock.NewController(t))
			if tt.verr != nil {
				v.EXPECT().Verify("tok").Return(domain.Claims{}, tt.verr)
			}

			rec := callValidate(v, tt.header)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if rec.Header().Get("X-User-Id") != "" {
				t.Fatal("identity headers must not be set on failure")
			}
		})
	}
}

func TestValidateHandler_Success(t *testing.T) {
	id := uuid.MustParse("019a0000-0000-7000-8000-000000000001")

	for _, header := range []string{"Bearer tok", "bearer tok", "BEARER tok"} {
		t.Run(header, func(t *testing.T) {
			v := mocks.NewMockAccessTokenVerifier(gomock.NewController(t))
			v.EXPECT().Verify("tok").Return(domain.Claims{UserID: id, Role: domain.RoleBusiness}, nil)

			rec := callValidate(v, header)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("X-User-Id"); got != id.String() {
				t.Errorf("X-User-Id = %q, want %q", got, id)
			}
			if got := rec.Header().Get("X-User-Role"); got != "business" {
				t.Errorf("X-User-Role = %q, want business", got)
			}
		})
	}
}
