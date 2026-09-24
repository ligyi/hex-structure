package grpc

import (
	userv1 "github.com/samverrall/hex-structure/gen/user/v1"
	"github.com/samverrall/hex-structure/internal/adapters/primary/grpc/users"
)

func (a *App) initAppRoutes() {
	userv1.RegisterUserServiceServer(a.grpc, users.NewUserService(a.userAPI))
}
