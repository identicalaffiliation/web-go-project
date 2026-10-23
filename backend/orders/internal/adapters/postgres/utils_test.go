//go:build unit

package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestValidateConstraints(t *testing.T) {
	t.Run("check violation", func(t *testing.T) {
		require.ErrorIs(t, validateConstraints(&pgconn.PgError{Code: checkCode}), ErrInvalidData)
	})

	t.Run("unique violation", func(t *testing.T) {
		require.ErrorIs(t, validateConstraints(&pgconn.PgError{Code: uniqueCode}), ErrAlreadyExists)
	})

	t.Run("other pg error is wrapped", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "42P01"}

		err := validateConstraints(pgErr)
		require.ErrorIs(t, err, pgErr)
		require.NotErrorIs(t, err, ErrInvalidData)
		require.NotErrorIs(t, err, ErrAlreadyExists)
	})

	t.Run("non pg error is wrapped", func(t *testing.T) {
		netErr := errors.New("connection refused")

		require.ErrorIs(t, validateConstraints(netErr), netErr)
	})
}
