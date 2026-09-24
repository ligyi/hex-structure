package grpc_test

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	userv1 "github.com/samverrall/hex-structure/gen/user/v1"
	grpcadapter "github.com/samverrall/hex-structure/internal/adapters/primary/grpc"
	"github.com/samverrall/hex-structure/internal/core/domain/user"
	"github.com/samverrall/hex-structure/internal/core/services/users"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type userRepo struct {
	users []user.User
}

func (r *userRepo) Add(_ context.Context, u user.User) error {
	r.users = append(r.users, u)
	return nil
}

// newConn starts the gRPC primary adapter over an in-memory listener, wired the
// same way cmd/api wires it (reflection included), and returns a connection to
// it plus the repo the calls land in.
func newConn(t *testing.T) (*grpc.ClientConn, *userRepo) {
	t.Helper()

	repo := &userRepo{}
	app := grpcadapter.NewApp(users.NewService(repo), grpcadapter.WithReflection())

	lis := bufconn.Listen(bufSize)
	go func() {
		if err := app.Serve(lis); err != nil {
			t.Logf("grpc server stopped: %v", err)
		}
	}()

	t.Cleanup(func() {
		app.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial the in-memory server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn, repo
}

// newClient is newConn plus the generated user service client.
func newClient(t *testing.T) (userv1.UserServiceClient, *userRepo) {
	t.Helper()

	conn, repo := newConn(t)

	return userv1.NewUserServiceClient(conn), repo
}

func TestCreateAccountOverGRPC(t *testing.T) {
	client, repo := newClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.CreateAccount(ctx, &userv1.CreateAccountRequest{Username: "alice"})
	if err != nil {
		t.Fatalf("CreateAccount returned an error: %v", err)
	}

	if len(repo.users) != 1 {
		t.Fatalf("repo holds %d users, want 1", len(repo.users))
	}

	if resp.GetUserId() != repo.users[0].ID.String() {
		t.Fatalf("response user ID = %q, stored user ID = %q", resp.GetUserId(), repo.users[0].ID)
	}

	if repo.users[0].Username != user.Username("alice") {
		t.Fatalf("stored username = %q, want %q", repo.users[0].Username, "alice")
	}
}

func TestCreateAccountRejectsInvalidUsername(t *testing.T) {
	client, _ := newClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.CreateAccount(ctx, &userv1.CreateAccountRequest{Username: "ab"})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("CreateAccount(%q) code = %v, want %v (err: %v)", "ab", got, codes.InvalidArgument, err)
	}
}

// TestReflectionExposesUserService guards the grpcui/grpcurl workflow: the
// adapter has to register the reflection service, otherwise those tools cannot
// discover the API without being handed the .proto file.
func TestReflectionExposesUserService(t *testing.T) {
	conn, _ := newConn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("opening the reflection stream: %v", err)
	}
	defer func() { _ = stream.CloseSend() }()

	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{ListServices: ""},
	}); err != nil {
		t.Fatalf("sending the list services request: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("receiving the list services response: %v", err)
	}

	var names []string
	for _, svc := range resp.GetListServicesResponse().GetService() {
		names = append(names, svc.GetName())
	}

	if !slices.Contains(names, "user.v1.UserService") {
		t.Fatalf("reflection listed %v, want it to contain %q", names, "user.v1.UserService")
	}
}
