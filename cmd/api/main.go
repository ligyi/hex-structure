package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	grpcadapter "github.com/samverrall/hex-structure/internal/adapters/primary/grpc"
	"github.com/samverrall/hex-structure/internal/adapters/primary/web"
	"github.com/samverrall/hex-structure/internal/adapters/secondary/postgres"
	"github.com/samverrall/hex-structure/internal/core/services/users"
)

const (
	httpPort = 8000
	grpcPort = 50051
)

func main() {
	// Cancel the context on ctrl-c / SIGTERM to drain both servers.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialise secondary port implementations (Secondary adapters)
	userRepo, err := postgres.NewUserRepo() // <- this is swappable since its just a repo implementation
	if err != nil {
		log.Fatalf("failed to init postgres user repo: %v", err)
	}

	// Initialise core service layer
	usersService := users.NewService(userRepo) // core business logic doesn't change.

	// Init primary (driving) adapters.
	// both transports drive the same service instance, so business logic stays
	// in one place no matter which adapter a client talks to.
	httpSrv := web.NewApp(usersService, web.WithPort(httpPort))
	grpcSrv := grpcadapter.NewApp(usersService, grpcadapter.WithPort(grpcPort))

	// Run each server blocks until it stops, so serve them concurrently.
	// The buffered channel means the second server never leaks a goroutine.
	errCh := make(chan error, 2)

	go func() {
		log.Printf("http primary adapter listening on :%d", httpPort)
		errCh <- httpSrv.Run()
	}()

	go func() {
		log.Printf("grpc primary adapter listening on :%d", grpcPort)
		errCh <- grpcSrv.Run()
	}()

	select {
	case <-ctx.Done():
		log.Print("shutdown signal received, draining connections")
	case err := <-errCh:
		if err != nil {
			log.Printf("server stopped: %v", err)
		}
	}

	// Stop accepting new calls and let in-flight ones finish.
	grpcSrv.Stop()
	if err := httpSrv.Shutdown(); err != nil {
		log.Printf("http shutdown failed: %v", err)
	}
}
