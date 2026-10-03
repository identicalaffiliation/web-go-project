package idgen

import "github.com/google/uuid"

type UUIDGenerator struct{}

func New() UUIDGenerator {
	return UUIDGenerator{}
}

func (UUIDGenerator) NewID() string {
	return uuid.NewString()
}
