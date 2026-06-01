package repository

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("repository: record not found")
	ErrConflict = errors.New("repository: conflict or unique constraint violation")
)

// mapError converts database-specific errors (like GORM errors) into standard repository errors.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
