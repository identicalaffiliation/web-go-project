//go:build unit

package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRoleMiddleware(t *testing.T) {
	cases := []struct {
		name string
		role string
		code int
	}{
		{"no role header - 401", "", http.StatusUnauthorized},
		{"role not allowed - 403", "admin", http.StatusForbidden},
		{"customer allowed", string(Customer), http.StatusOK},
		{"business allowed", string(Business), http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()

			var gotRole any
			e.GET("/", func(ctx echo.Context) error {
				gotRole = ctx.Get(CtxRoleKey)
				return ctx.NoContent(http.StatusOK)
			}, RoleMiddleware(Customer, Business))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.role != "" {
				req.Header.Set(HeaderUserRole, tc.role)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tc.code, rec.Code)
			if tc.code == http.StatusOK {
				require.Equal(t, Role(tc.role), gotRole)
			}
		})
	}
}
