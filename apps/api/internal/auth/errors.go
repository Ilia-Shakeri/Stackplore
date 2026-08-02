package auth

import "errors"

var (
	ErrInvalidPassword           = errors.New("password input is invalid")
	ErrInvalidPasswordPolicy     = errors.New("password policy is invalid")
	ErrInvalidPasswordCredential = errors.New("password credential is invalid")
	ErrPasswordHashUnavailable   = errors.New("password hashing is unavailable")
)
