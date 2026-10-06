package utils

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	checkCode = "23514"
)

func IsCheckConstraint(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return strings.EqualFold(checkCode, pgErr.Code)
	}

	return false
}
