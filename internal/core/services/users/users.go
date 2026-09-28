package users

import (
	"context"

	"github.com/ligyi/hex-structure/internal/ports"
)

type API interface {
	CreateAccount(context.Context, CreateAccountReq) (*CreateAccountResp, error)
}

type Service struct {
	userRepo ports.UserRepo
}

func NewService(ur ports.UserRepo) *Service {
	return &Service{
		userRepo: ur,
	}
}
