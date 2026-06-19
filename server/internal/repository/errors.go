package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return ErrConflict
		}
		if pgErr.Code == "22P02" {
			return ErrNotFound
		}
	}
	return err
}
