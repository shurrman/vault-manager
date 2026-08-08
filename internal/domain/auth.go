package domain

import (
	"context"
	"errors"
)

type AuthMethod string

const (
	AuthUserpass AuthMethod = "userpass"
	AuthToken    AuthMethod = "token"
)

type LoginRequest struct {
	Address  string
	Method   AuthMethod
	Username string
	Password string
	Token    string
}

type Session struct {
	Address     string
	DisplayName string
}

type AuthBackend interface {
	Backend
	Address() string
	Session(context.Context) (Session, error)
	Login(context.Context, LoginRequest) (Session, error)
}

type AuthenticationError struct {
	Err error
}

func (e *AuthenticationError) Error() string {
	return e.Err.Error()
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

func IsAuthenticationError(err error) bool {
	var target *AuthenticationError
	return errors.As(err, &target)
}
