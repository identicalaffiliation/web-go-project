package domain

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var Validate = sync.OnceValue[*validator.Validate](func() *validator.Validate {
	return validator.New()
})
