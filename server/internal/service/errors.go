package service

import "fmt"

// ErrValidation returns a validation error.
func ErrValidation(msg string) error {
	return fmt.Errorf("validation: %s", msg)
}
