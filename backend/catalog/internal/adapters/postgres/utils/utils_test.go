package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsCheckConstraint(t *testing.T) {
	err := &pgconn.PgError{Code: checkCode}
	require.NotNil(t, err)
	require.Equal(t, true, IsCheckConstraint(err))
}

func TestConstraintMatchers(t *testing.T) {
	check := &pgconn.PgError{Code: checkCode}
	foreign := &pgconn.PgError{Code: foreignCode}

	// репозитории оборачивают ошибки через %w - матчинг должен это переживать
	require.True(t, IsCheckConstraint(fmt.Errorf("insert: %w", check)))
	require.True(t, IsForeignConstraint(fmt.Errorf("insert: %w", foreign)))

	require.False(t, IsCheckConstraint(foreign))
	require.False(t, IsForeignConstraint(check))

	require.False(t, IsCheckConstraint(errors.New("plain")))
	require.False(t, IsForeignConstraint(errors.New("plain")))
	require.False(t, IsForeignConstraint(nil))
}
