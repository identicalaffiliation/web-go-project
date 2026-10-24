//go:build unit

package rpc

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/identicalaffiliation/web-go-project/backend/payments/gen/mocks"
	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/balance"
)

func TestHandler_GetBalanceByUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.NewV7().String()
	req := &balancev1.GetBalanceRequest{UserId: userID}

	t.Run("returns balance", func(t *testing.T) {
		svc := mocks.NewMockBalanceService(t)
		want := &balancev1.GetBalanceResponse{UserId: userID, Amount: 1_000_000}
		svc.EXPECT().GetBalance(mock.Anything, userID).Return(want, nil).Once()

		resp, err := NewRPChandler(svc).GetBalanceByUser(ctx, req)
		require.NoError(t, err)
		require.Equal(t, want, resp)
	})

	cases := map[string]struct {
		err  error
		code codes.Code
	}{
		"invalid user id":   {balance.ErrInvalidData, codes.InvalidArgument},
		"account not found": {domain.ErrAccountNotFound, codes.NotFound},
		"internal error":    {errors.New("db down"), codes.Internal},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc := mocks.NewMockBalanceService(t)
			svc.EXPECT().GetBalance(mock.Anything, userID).Return(nil, tc.err).Once()

			_, err := NewRPChandler(svc).GetBalanceByUser(ctx, req)
			require.Equal(t, tc.code, status.Code(err))
		})
	}
}
