package rest

import (
	"errors"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator проверяет теги validate на DTO. Реализует echo.Validator, поэтому
// хендлеры вызывают c.Validate(&req), не зная о библиотеке.
type Validator struct {
	v *validator.Validate
}

// NewValidator создаёт валидатор один раз при старте. validator.Validate
// потокобезопасен и кеширует разбор тегов по типам, поэтому один экземпляр
// на все запросы быстрее, чем создавать его на каждый.
func NewValidator() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Имена полей в ошибках берём из json-тегов: клиент знает "email", а не "Email".
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	return &Validator{v: v}
}

func (val *Validator) Validate(i any) error {
	return val.v.Struct(i)
}

// validationMessage превращает ошибку валидатора в короткий текст для клиента.
// Показываем только имя поля и тип нарушения, без значения и внутренних деталей
// библиотеки. Любая другая ошибка (например, не тот тип в Validate) — внутренняя,
// её клиенту не раскрываем.
func validationMessage(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) || len(verrs) == 0 {
		return "invalid request"
	}

	fe := verrs[0]
	if fe.Tag() == "required" {
		return fe.Field() + " is required"
	}
	return "invalid " + fe.Field()
}
