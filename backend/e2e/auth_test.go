//go:build integration

package e2e

import (
	"net/http"
	"testing"
)

const profilePath = "/api/v1/profiles/me"

func TestAuth(t *testing.T) {
	api := newClient(t)
	token := login(t, api)

	api.request(t, http.MethodGet, profilePath, "", nil, http.StatusUnauthorized, nil)
	api.request(t, http.MethodGet, profilePath, token, nil, http.StatusOK, nil)
}

func login(t *testing.T, api client) string {
	t.Helper()
	credentials := map[string]string{
		"email":    requiredEnv(t, "E2E_EMAIL"),
		"password": requiredEnv(t, "E2E_PASSWORD"),
	}
	var tokens struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	api.request(t, http.MethodPost, "/api/v1/auth/login", "", credentials, http.StatusOK, &tokens)
	if tokens.Access == "" || tokens.Refresh == "" {
		t.Fatal("login did not return both tokens")
	}
	return tokens.Access
}
