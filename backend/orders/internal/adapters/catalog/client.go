package catalog

import (
	"context"
	"fmt"
	"uuid"

	pricev1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/price/v1"
	"google.golang.org/grpc"
)

type Client struct {
	api pricev1.PriceServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{api: pricev1.NewPriceServiceClient(conn)}
}

func (c *Client) GetPrice(ctx context.Context, productID uuid.UUID) (int64, error) {
	resp, err := c.api.GetPriceByProductId(ctx, &pricev1.PriceRequest{ProductId: productID.String()})
	if err != nil {
		return 0, fmt.Errorf("get price: %w", err)
	}

	return resp.GetPrice(), nil
}
