package utils

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsCheckConstraint(t *testing.T) {
	err := &pgconn.PgError{Code: checkCode}
	require.NotNil(t, err)
	require.Equal(t, true, IsCheckConstraint(err))
}
