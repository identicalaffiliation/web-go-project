package rest

import "time"

// Теги проверяют только форму запроса. Сложность пароля (длина в символах и
// байтах, пробелы) проверяет domain.ValidatePassword.
//
// Теги example/minLength/maxLength нужны только swagger: в примерах заглушки,
// не настоящие данные.
type registerRequest struct {
	Email    string `json:"email" validate:"required,email" format:"email" example:"user@example.com"`
	Password string `json:"password" validate:"required" minLength:"8" maxLength:"72" example:"your-password-here"`
}

type registerResponse struct {
	ID        string    `json:"id" format:"uuid" example:"0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"`
	Email     string    `json:"email" format:"email" example:"user@example.com"`
	Role      string    `json:"role" enums:"customer,business" example:"customer"`
	CreatedAt time.Time `json:"created_at"`
}

// При логине пароль только required: правила сложности относятся к регистрации,
// а старый пароль может им не соответствовать.
type loginRequest struct {
	Email    string `json:"email" validate:"required,email" format:"email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"your-password-here"`
}

type tokenPairResponse struct {
	AccessToken      string    `json:"access_token" example:"<access-jwt>"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token" example:"<refresh-token>"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"<refresh-token>"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" example:"<refresh-token>"`
}

type errorResponse struct {
	Error string `json:"error" example:"invalid request body"`
}
