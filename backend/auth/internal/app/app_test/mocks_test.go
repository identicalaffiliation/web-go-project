package app_test

import (
	"context"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type userRepoMock struct {
	createFn     func(ctx context.Context, u domain.User) error
	getByEmailFn func(ctx context.Context, email domain.Email) (domain.User, error)
	getByIDFn    func(ctx context.Context, id domain.UserID) (domain.User, error)
}

func (m *userRepoMock) Create(ctx context.Context, u domain.User) error {
	return m.createFn(ctx, u)
}

func (m *userRepoMock) GetByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	return m.getByEmailFn(ctx, email)
}

func (m *userRepoMock) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	return m.getByIDFn(ctx, id)
}

type refreshRepoMock struct {
	createFn    func(ctx context.Context, t domain.RefreshToken) error
	getByHashFn func(ctx context.Context, hash string) (domain.RefreshToken, error)
	rotateFn    func(ctx context.Context, oldID string, next domain.RefreshToken, now time.Time) error
	revokeFn    func(ctx context.Context, id string, now time.Time) error
}

func (m *refreshRepoMock) Create(ctx context.Context, t domain.RefreshToken) error {
	return m.createFn(ctx, t)
}

func (m *refreshRepoMock) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	return m.getByHashFn(ctx, hash)
}

func (m *refreshRepoMock) Rotate(ctx context.Context, oldID string, next domain.RefreshToken, now time.Time) error {
	return m.rotateFn(ctx, oldID, next, now)
}

func (m *refreshRepoMock) Revoke(ctx context.Context, id string, now time.Time) error {
	return m.revokeFn(ctx, id, now)
}

type hasherMock struct {
	hashFn    func(plain string) (string, error)
	compareFn func(hash, plain string) error
}

func (m hasherMock) Hash(plain string) (string, error) { return m.hashFn(plain) }
func (m hasherMock) Compare(hash, plain string) error  { return m.compareFn(hash, plain) }

type issuerMock struct {
	issueFn func(claims domain.Claims) (string, time.Time, error)
}

func (m issuerMock) Issue(claims domain.Claims) (string, time.Time, error) { return m.issueFn(claims) }

type verifierMock struct {
	verifyFn func(token string) (domain.Claims, error)
}

func (m verifierMock) Verify(token string) (domain.Claims, error) { return m.verifyFn(token) }

type tokenGenMock struct {
	generateFn func() (plain, hash string, err error)
	hashFn     func(plain string) string
}

func (m tokenGenMock) Generate() (string, string, error) { return m.generateFn() }
func (m tokenGenMock) Hash(plain string) string          { return m.hashFn(plain) }

type clockMock struct{ nowFn func() time.Time }

func (m clockMock) Now() time.Time { return m.nowFn() }

type idGenMock struct{ newIDFn func() string }

func (m idGenMock) NewID() string { return m.newIDFn() }

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)
