package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	userv1 "github.com/samverrall/hex-structure/gen/user/v1"
	grpcadapter "github.com/samverrall/hex-structure/internal/adapters/primary/grpc"
	"github.com/samverrall/hex-structure/internal/core/domain/user"
	"github.com/samverrall/hex-structure/internal/core/services/users"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
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

// newClient starts the gRPC primary adapter over an in-memory listener and
// returns a client for it together with the repo the calls land in.
func newClient(t *testing.T) (userv1.UserServiceClient, *userRepo) {
	t.Helper()

	repo := &userRepo{}
	app := grpcadapter.NewApp(users.NewService(repo))

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
