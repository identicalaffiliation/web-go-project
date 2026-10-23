//go:build unit

package domain

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestAccount_ToResponseRPC(t *testing.T) {
	account := &Account{UserID: uuid.NewV7(), Balance: 1_000_000}

	resp := account.ToResponseRPC()
	require.Equal(t, account.UserID.String(), resp.GetUserId())
	require.Equal(t, int64(1_000_000), resp.GetAmount())
}
