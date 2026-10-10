package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type mockAuthService struct {
	registerFunc func(ctx context.Context, in ports.RegisterInput) (domain.User, error)
	loginFunc    func(ctx context.Context, in ports.LoginInput) (domain.TokenPair, error)
}

func (m *mockAuthService) Register(ctx context.Context, in ports.RegisterInput) (domain.User, error) {
	return m.registerFunc(ctx, in)
}

func (m *mockAuthService) Login(ctx context.Context, in ports.LoginInput) (domain.TokenPair, error) {
	return m.loginFunc(ctx, in)
}

func (m *mockAuthService) Refresh(context.Context, string) (domain.TokenPair, error) {
	return domain.TokenPair{}, nil
}
func (m *mockAuthService) Logout(context.Context, string) error { return nil }
func (m *mockAuthService) ValidateToken(string) (domain.Claims, error) {
	return domain.Claims{}, nil
}

func newTestEcho(svc ports.AuthService) *echo.Echo {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rest.NewHandler(svc, logger).Register(e.Group("/api/v1/auth"))
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
	svc := &mockAuthService{
		registerFunc: func(_ context.Context, in ports.RegisterInput) (domain.User, error) {
			return domain.User{ID: "user-1", Email: domain.Email(in.Email), Role: domain.RoleCustomer, CreatedAt: time.Now()}, nil
		},
	}
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "secret123"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestHandler_Register_EmailTaken(t *testing.T) {
	svc := &mockAuthService{
		registerFunc: func(context.Context, ports.RegisterInput) (domain.User, error) {
			return domain.User{}, domain.ErrEmailTaken
		},
	}
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "secret123"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	svc := &mockAuthService{
		loginFunc: func(context.Context, ports.LoginInput) (domain.TokenPair, error) {
			return domain.TokenPair{}, domain.ErrInvalidCredentials
		},
	}
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "a@example.com", "password": "wrong"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
