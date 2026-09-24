package users

import (
	"context"
	"testing"

	"github.com/samverrall/hex-structure/internal/core/domain/user"
)

type userRepo struct {
	users []user.User
}

func (r *userRepo) Add(_ context.Context, u user.User) error {
	r.users = append(r.users, u)
	return nil
}

func TestCreateAccount(t *testing.T) {
	repo := &userRepo{}
	svc := NewService(repo)

	resp, err := svc.CreateAccount(context.Background(), CreateAccountReq{Username: "alice"})
	if err != nil {
		t.Fatalf("CreateAccount returned an error: %v", err)
	}

	if resp.UserID != repo.users[0].ID.String() {
		t.Fatalf("response user ID = %q, stored user ID = %q", resp.UserID, repo.users[0].ID)
	}

	if repo.users[0].Username != user.Username("alice") {
		t.Fatalf("stored username = %q, want %q", repo.users[0].Username, "alice")
	}
}
