package grpc

import (
	"fmt"
	"net"

	"github.com/samverrall/hex-structure/internal/core/services/users"
	"google.golang.org/grpc"
)

type App struct {
	grpc       *grpc.Server
	userAPI    users.API
	port       int
	serverOpts []grpc.ServerOption
}

func NewApp(userAPI users.API, opts ...AppOption) *App {
	s := &App{
		userAPI: userAPI,
		port:    50051,
	}

	// Options are applied before the server is built so that server options
	// (interceptors, credentials, ...) are taken into account.
	for _, applyOption := range opts {
		applyOption(s)
	}

	s.grpc = grpc.NewServer(s.serverOpts...)

	s.initAppRoutes()

	return s
}

// Run listens on the configured port and blocks until the server stops.
func (a *App) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", a.port, err)
	}

	return a.Serve(lis)
}

// Serve blocks serving RPCs on lis. It is used by Run and lets callers inject
// their own listener (e.g. bufconn in tests).
func (a *App) Serve(lis net.Listener) error {
	return a.grpc.Serve(lis)
}

// Stop gracefully stops the server, draining in-flight RPCs.
func (a *App) Stop() {
	a.grpc.GracefulStop()
}
