package user_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/samverrall/hex-structure/internal/core/domain/user"
)

func TestNewUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    user.Username
		wantErr error
	}{
		{
			name:  "valid minimum length",
			input: "alice",
			want:  "alice",
		},
		{
			name:  "trims surrounding space",
			input: "  alice  ",
			want:  "alice",
		},
		{
			name:  "allows letters digits underscore and hyphen",
			input: "Alice_01-ok",
			want:  "Alice_01-ok",
		},
		{
			name:  "allows max length",
			input: strings.Repeat("a", 100),
			want:  user.Username(strings.Repeat("a", 100)),
		},
		{
			name:    "rejects empty",
			input:   "",
			wantErr: user.ErrEmptyUsername,
		},
		{
			name:    "rejects whitespace only",
			input:   "   ",
			wantErr: user.ErrEmptyUsername,
		},
		{
			name:    "rejects shorter than 5",
			input:   "abcd",
			wantErr: user.ErrUsernameLength,
		},
		{
			name:    "rejects longer than 100",
			input:   strings.Repeat("a", 101),
			wantErr: user.ErrUsernameLength,
		},
		{
			name:    "rejects spaces inside",
			input:   "alice bob",
			wantErr: user.ErrUsernameCharset,
		},
		{
			name:    "rejects punctuation",
			input:   "alice@bob",
			wantErr: user.ErrUsernameCharset,
		},
		{
			name:    "rejects symbols",
			input:   "alice!",
			wantErr: user.ErrUsernameCharset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := user.NewUsername(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewUsername(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewUsername(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NewUsername(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
