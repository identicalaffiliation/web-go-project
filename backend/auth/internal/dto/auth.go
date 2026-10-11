package dto

type RegisterInput struct {
	Email    string
	Password string
	Role     string
}

type LoginInput struct {
	Email    string
	Password string
}
