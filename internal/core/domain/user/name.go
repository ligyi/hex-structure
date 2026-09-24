package user

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minUsernameLen = 5
	maxUsernameLen = 100
)

var (
	ErrEmptyUsername   = errors.New("empty username supplied")
	ErrUsernameLength  = errors.New("username must be between 5 and 100 characters")
	ErrUsernameCharset = errors.New("username contains invalid characters")
)

type Username string

func NewUsername(un string) (Username, error) {
	un = strings.TrimSpace(un)
	if un == "" {
		return "", ErrEmptyUsername
	}

	if n := utf8.RuneCountInString(un); n < minUsernameLen || n > maxUsernameLen {
		return "", ErrUsernameLength
	}

	if !isValidUsername(un) {
		return "", ErrUsernameCharset
	}

	return Username(un), nil
}

// isValidUsername allows letters, digits, and the separators '_' and '-'.
// Spaces and other specific characters are rejected.
func isValidUsername(un string) bool {
	for _, r := range un {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
