package grpc

import "google.golang.org/grpc"

type AppOption func(a *App)

// WithPort applies an optional port to the server.
func WithPort(port int) AppOption {
	return func(a *App) {
		a.port = port
	}
}

// WithServerOptions appends options to the underlying grpc.Server.
func WithServerOptions(opts ...grpc.ServerOption) AppOption {
	return func(a *App) {
		a.serverOpts = append(a.serverOpts, opts...)
	}
}

// WithReflection registers the gRPC server reflection service, which lets tools
// such as grpcurl and grpcui discover the API without being handed the .proto
// file. Convenient in development; consider dropping it (or gating it behind
// config) for internet-facing deployments.
func WithReflection() AppOption {
	return func(a *App) {
		a.reflection = true
	}
}
