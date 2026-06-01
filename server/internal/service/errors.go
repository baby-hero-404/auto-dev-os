package service

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrAuthorization = errors.New("authorization")
	ErrInvalid       = errors.New("validation")
)

type DomainError struct {
	Kind error
	Msg  string
}

func (e DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Kind, e.Msg)
}

func (e DomainError) Unwrap() error {
	return e.Kind
}

// ErrValidation returns a validation error.
func ErrValidation(msg string) error {
	return DomainError{Kind: ErrInvalid, Msg: msg}
}

func ErrNotFoundf(msg string) error {
	return DomainError{Kind: ErrNotFound, Msg: msg}
}

func ErrConflictf(msg string) error {
	return DomainError{Kind: ErrConflict, Msg: msg}
}

func ErrAuthorizationf(msg string) error {
	return DomainError{Kind: ErrAuthorization, Msg: msg}
}
