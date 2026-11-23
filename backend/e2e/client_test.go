//go:build integration

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type client struct {
	baseURL string
	http    *http.Client
}

func newClient(t *testing.T) client {
	t.Helper()
	return client{
		baseURL: strings.TrimRight(requiredEnv(t, "E2E_BASE_URL"), "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("set %s before running e2e tests", name)
	}
	return value
}

func (c client) request(t *testing.T, method, path, token string, input any, status int, output any) {
	t.Helper()

	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, c.baseURL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	if res.StatusCode != status {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		t.Fatalf("%s %s: got HTTP %d, want %d (%s)", method, path, res.StatusCode, status, message)
	}
	if output != nil {
		if err := json.NewDecoder(res.Body).Decode(output); err != nil {
			t.Fatalf("%s %s: decode response: %v", method, path, err)
		}
	}
}
