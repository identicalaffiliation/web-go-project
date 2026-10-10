package middlewares

import "github.com/labstack/echo"

const (
	Business Role = "business"
	Client   Role = "client"
)

const (
	HeaderUserRole = "X-User-Role"
	CtxRoleKey     = "role"
)

type Role string

func RoleMiddleware(roles ...Role) echo.MiddlewareFunc {
	allowed := make(map[Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if r := ctx.Request().Header.Get(HeaderUserRole); r == "" {
				return echo.ErrUnauthorized
			}

			role := Role(ctx.Request().Header.Get(HeaderUserRole))
			if _, ok := allowed[role]; !ok {
				return echo.ErrForbidden
			}

			ctx.Set(CtxRoleKey, role)
			return next(ctx)
		}
	}
}
