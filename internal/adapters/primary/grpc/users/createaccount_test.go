package users

import (
	"context"
	"errors"
	"fmt"
	"testing"

	userv1 "github.com/samverrall/hex-structure/gen/user/v1"
	"github.com/samverrall/hex-structure/internal/core/domain/user"
	"github.com/samverrall/hex-structure/internal/core/services/users"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// apiStub is a stand-in for the inbound port so the transport can be tested
// without the real service or a database.
type apiStub struct {
	users.CreateAccountResp

	gotReq users.CreateAccountReq
	err    error
}

func (s *apiStub) CreateAccount(_ context.Context, req users.CreateAccountReq) (*users.CreateAccountResp, error) {
	s.gotReq = req
	if s.err != nil {
		return nil, s.err
	}
	return &s.CreateAccountResp, nil
}

func TestCreateAccount(t *testing.T) {
	const userID = "1b4e28ba-2fa1-11d2-883f-0016d3cca427"

	tests := []struct {
		name     string
		username string
		api      *apiStub
		wantCode codes.Code
		wantID   string
	}{
		{
			name:     "returns the user id from the application service",
			username: "alice",
			api:      &apiStub{CreateAccountResp: users.CreateAccountResp{UserID: userID}},
			wantCode: codes.OK,
			wantID:   userID,
		},
		{
			name:     "maps an invalid username to InvalidArgument",
			username: "ab",
			api:      &apiStub{err: fmt.Errorf("invalid username supplied: %w", user.ErrUsernameLength)},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "maps an unexpected failure to Internal",
			username: "alice",
			api:      &apiStub{err: errors.New("failed to add a user: connection refused")},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(tt.api)

			resp, err := svc.CreateAccount(context.Background(), &userv1.CreateAccountRequest{Username: tt.username})
			if got := status.Code(err); got != tt.wantCode {
				t.Fatalf("CreateAccount(%q) code = %v, want %v (err: %v)", tt.username, got, tt.wantCode, err)
			}
			if tt.wantCode != codes.OK {
				return
			}
			if resp.GetUserId() != tt.wantID {
				t.Fatalf("response user ID = %q, want %q", resp.GetUserId(), tt.wantID)
			}
			if tt.api.gotReq.Username != tt.username {
				t.Fatalf("service received username = %q, want %q", tt.api.gotReq.Username, tt.username)
			}
		})
	}
}
