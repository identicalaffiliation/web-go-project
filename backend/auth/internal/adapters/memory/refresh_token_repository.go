package memory

import (
	"context"
	"sync"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type RefreshTokenRepository struct {
	mu     sync.Mutex
	tokens map[string]domain.RefreshToken // token_hash -> token
}

func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{tokens: make(map[string]domain.RefreshToken)}
}

func (r *RefreshTokenRepository) Create(_ context.Context, token domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = token
	return nil
}

func (r *RefreshTokenRepository) GetByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tokens[hash]
	if !ok {
		return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (r *RefreshTokenRepository) Rotate(_ context.Context, oldID string, next domain.RefreshToken, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for hash, t := range r.tokens {
		if t.ID == oldID {
			t.RevokedAt = &now
			r.tokens[hash] = t
		}
	}
	r.tokens[next.TokenHash] = next
	return nil
}

func (r *RefreshTokenRepository) Revoke(_ context.Context, id string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for hash, t := range r.tokens {
		if t.ID == id {
			t.RevokedAt = &now
			r.tokens[hash] = t
		}
	}
	return nil
}

var _ ports.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
