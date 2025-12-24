package contracts

import (
	"context"

	"siaga-api/api/entities"
)

type AuthRepository interface {
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
}

type AuthLoginResult struct {
	AccessToken string         `json:"access_token"`
	User        *entities.User `json:"user"`
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (*AuthLoginResult, error)
	LoginAdmin(ctx context.Context, email, password string) (*AuthLoginResult, error)
}
