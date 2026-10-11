package app_test

import (
	"time"

	"github.com/google/uuid"
)

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)

var (
	testUserID  = uuid.MustParse("019a0000-0000-7000-8000-000000000001")
	testTokenID = uuid.MustParse("019a0000-0000-7000-8000-000000000002")
)
