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

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
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
	log := logging.NewNop()
	g := e.Group("/api/v1/auth")
	g.POST("/register", rest.RegisterHandler(svc, log))
	g.POST("/login", rest.LoginHandler(svc, log))
	g.POST("/refresh", rest.RefreshHandler(svc, log))
	g.POST("/logout", rest.LogoutHandler(svc, log))
	g.POST("/validate", rest.ValidateHandler(svc, log))
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
			return domain.User{ID: uuid.MustParse("019a0000-0000-7000-8000-000000000001"), Email: domain.Email(in.Email), Role: domain.RoleCustomer, CreatedAt: time.Now()}, nil
		},
	}
	rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "a@example.com", "password": "secret123", "role": "business"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestHandler_Register_InvalidRole(t *testing.T) {
	svc := &mockAuthService{
		registerFunc: func(context.Context, ports.RegisterInput) (domain.User, error) {
			return domain.User{}, domain.ErrInvalidRole
		},
	}
	// Поле role отсутствует, пустое или admin: во всех случаях 400.
	bodies := map[string]map[string]string{
		"missing": {"email": "a@example.com", "password": "secret123"},
		"empty":   {"email": "a@example.com", "password": "secret123", "role": ""},
		"admin":   {"email": "a@example.com", "password": "secret123", "role": "admin"},
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(newTestEcho(svc), http.MethodPost, "/api/v1/auth/register", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
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
