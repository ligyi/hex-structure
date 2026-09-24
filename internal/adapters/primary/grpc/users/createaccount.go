package users

import (
	"context"

	userv1 "github.com/samverrall/hex-structure/gen/user/v1"
	"github.com/samverrall/hex-structure/internal/core/services/users"
)

// UserService adapts the users application API (inbound port) to the generated
// gRPC contract.
type UserService struct {
	userv1.UnimplementedUserServiceServer
	userAPI users.API
}

func NewUserService(userAPI users.API) *UserService {
	return &UserService{
		userAPI: userAPI,
	}
}

func (s *UserService) CreateAccount(ctx context.Context, req *userv1.CreateAccountRequest) (*userv1.CreateAccountResponse, error) {
	resp, err := s.userAPI.CreateAccount(ctx, users.CreateAccountReq{Username: req.GetUsername()})
	if err != nil {
		return nil, toStatusError(err)
	}

	return &userv1.CreateAccountResponse{UserId: resp.UserID}, nil
}
