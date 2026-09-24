package users

import (
	"errors"

	"github.com/samverrall/hex-structure/internal/core/domain/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toStatusError translates application errors into gRPC status errors so the
// transport speaks the status codes clients expect.
func toStatusError(err error) error {
	switch {
	case errors.Is(err, user.ErrEmptyUsername),
		errors.Is(err, user.ErrUsernameLength),
		errors.Is(err, user.ErrUsernameCharset):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
