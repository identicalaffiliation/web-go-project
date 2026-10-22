package main

// Общие аннотации swag лежат отдельно от main.go: swag берёт их из любого
// комментария файла, указанного в -g (см. make swag-auth), а main.go не
// разрастается.
//
// @title Auth Service API
// @version 1.0
// @description Registration, login and JWT access/refresh token management.
// @BasePath /api/v1/auth
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT.
