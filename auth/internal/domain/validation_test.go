package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    domain.Email
		wantErr bool
	}{
		{name: "normalizes case and spaces", in: "  Alice@Example.COM ", want: "alice@example.com"},
		{name: "empty", in: "", wantErr: true},
		{name: "no at sign", in: "alice.example.com", wantErr: true},
		{name: "with display name", in: "Alice <alice@example.com>", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NewEmail(tc.in)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidEmail) {
					t.Fatalf("want ErrInvalidEmail, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "ok", in: "12345678"},
		{name: "too short", in: "1234567", wantErr: true},
		{name: "too long", in: strings.Repeat("a", domain.MaxPasswordBytes+1), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePassword(tc.in)
			if tc.wantErr && !errors.Is(err, domain.ErrInvalidPassword) {
				t.Fatalf("want ErrInvalidPassword, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
