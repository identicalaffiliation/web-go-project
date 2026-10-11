package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func newTestEcho(svc ports.AuthService) *echo.Echo {
	e := echo.New()
	e.Validator = rest.NewValidator()
	log := logging.NewNop()
	g := e.Group("/api/v1/auth")
	g.POST("/register", rest.RegisterHandler(svc, log))
	g.POST("/login", rest.LoginHandler(svc, log))
	g.POST("/refresh", rest.RefreshHandler(svc, log))
	g.POST("/logout", rest.LogoutHandler(svc, log))
	return e
}

func doJSON(e *echo.Echo, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Register_Success(t *testing.T) {
	svc := mocks.NewMockAuthService(gomock.NewController(t))
	svc.EXPECT().Register(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in ports.RegisterInput) (domain.User, error) {
			return domain.User{ID: uuid.MustParse("019a0000-0000-7000-8000-000000000001"), Email: domain.Email(in.Email), Role: domain.RoleCustomer, CreatedAt: time.Now()}, nil
		})
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "secret123", "role": "business"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}


// failingService — мок без ожиданий: любой вызов сервиса валит тест.
func failingService(t *testing.T) *mocks.MockAuthService {
	t.Helper()
	return mocks.NewMockAuthService(gomock.NewController(t))
}

func TestHandler_Register_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]string
		wantMsg string
	}{
		{"empty email", map[string]string{"email": "", "password": "secret123", "role": "customer"}, "email is required"},
		{"missing email", map[string]string{"password": "secret123", "role": "customer"}, "email is required"},
		{"invalid email", map[string]string{"email": "not-an-email", "password": "secret123", "role": "customer"}, "invalid email"},
		{"empty password", map[string]string{"email": "a@example.com", "password": "", "role": "customer"}, "password is required"},
		{"missing role", map[string]string{"email": "a@example.com", "password": "secret123"}, "role is required"},
		{"empty role", map[string]string{"email": "a@example.com", "password": "secret123", "role": ""}, "role is required"},
		{"admin role", map[string]string{"email": "a@example.com", "password": "secret123", "role": "admin"}, "invalid role"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(newTestEcho(failingService(t)), http.MethodPost, "/api/v1/auth/register", tc.body)
			assertBadRequest(t, rec, tc.wantMsg)
		})
	}
}

func TestHandler_Register_DomainPasswordError(t *testing.T) {
	// Сложность пароля теги не проверяют: ошибку отдаёт сервис (domain.ValidatePassword).
	svc := mocks.NewMockAuthService(gomock.NewController(t))
	svc.EXPECT().Register(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrInvalidPassword)
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "short", "role": "customer"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandler_Login_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]string
		wantMsg string
	}{
		{"empty email", map[string]string{"email": "", "password": "x"}, "email is required"},
		{"invalid email", map[string]string{"email": "nope", "password": "x"}, "invalid email"},
		{"empty password", map[string]string{"email": "a@example.com", "password": ""}, "password is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(newTestEcho(failingService(t)), http.MethodPost, "/api/v1/auth/login", tc.body)
			assertBadRequest(t, rec, tc.wantMsg)
		})
	}
}

func assertBadRequest(t *testing.T, rec *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Error != wantMsg {
		t.Errorf("error = %q, want %q", resp.Error, wantMsg)
	}
}

func TestHandler_Register_EmailTaken(t *testing.T) {
	svc := mocks.NewMockAuthService(gomock.NewController(t))
	svc.EXPECT().Register(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "secret123", "role": "customer"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	svc := mocks.NewMockAuthService(gomock.NewController(t))
	svc.EXPECT().Login(gomock.Any(), gomock.Any()).Return(domain.TokenPair{}, domain.ErrInvalidCredentials)
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "a@example.com", "password": "wrong"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
