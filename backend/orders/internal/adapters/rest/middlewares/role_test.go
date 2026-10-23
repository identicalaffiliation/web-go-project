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
		name     string
		role     string
		wantCode int
	}{
		{name: "allowed role", role: string(Customer), wantCode: http.StatusOK},
		{name: "no role header", role: "", wantCode: http.StatusUnauthorized},
		{name: "forbidden role", role: "admin", wantCode: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.GET("/", func(ctx echo.Context) error {
				require.Equal(t, Customer, ctx.Get(CtxRoleKey))
				return ctx.NoContent(http.StatusOK)
			}, RoleMiddleware(Customer))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.role != "" {
				req.Header.Set(HeaderUserRole, tc.role)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tc.wantCode, rec.Code)
		})
	}
}
