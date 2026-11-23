//go:build integration

package e2e

import (
	"net/http"
	"testing"
	"time"
	"uuid"
)

func TestCheckout(t *testing.T) {
	api := newClient(t)
	productID, err := uuid.Parse(requiredEnv(t, "E2E_PRODUCT_ID"))
	if err != nil || productID == uuid.Nil() {
		t.Fatal("E2E_PRODUCT_ID must be the UUID of an in-stock product")
	}
	token := login(t, api)

	input := map[string]any{
		"productId":   productID.String(),
		"operationId": uuid.NewV4().String(),
		"quantity":    1,
	}
	var created struct {
		Order struct {
			ID        string `json:"id"`
			ProductID string `json:"productId"`
			Quantity  int    `json:"quantity"`
		} `json:"order"`
	}
	api.request(t, http.MethodPost, "/api/v1/orders", token, input, http.StatusCreated, &created)
	order := created.Order
	if order.ID == "" || order.ProductID != productID.String() || order.Quantity != 1 {
		t.Fatalf("unexpected order: %+v", order)
	}

	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		var orders []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		api.request(t, http.MethodGet, "/api/v1/profiles/orders", token, nil, http.StatusOK, &orders)
		for _, item := range orders {
			if item.ID != order.ID {
				continue
			}
			switch item.Status {
			case "PAID":
				return
			case "FAILED":
				t.Fatalf("order %s failed", order.ID)
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("order %s did not reach PAID within 30s", order.ID)
}
