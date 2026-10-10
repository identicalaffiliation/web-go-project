package domain

import "time"

type Claims struct {
	UserID UserID
	Email  Email
	Role   Role
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type RefreshToken struct {
	ID        string
	UserID    UserID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (t RefreshToken) CheckActive(now time.Time) error {
	if t.RevokedAt != nil {
		return ErrRefreshTokenRevoked
	}

	if !now.Before(t.ExpiresAt) {
		return ErrRefreshTokenExpired
	}

	return nil
}
