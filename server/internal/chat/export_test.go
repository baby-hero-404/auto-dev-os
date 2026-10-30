package chat

import (
	"testing"

	"gorm.io/gorm"
)

// OpenTestDB lets the external test package use the Postgres test database.
func OpenTestDB(t *testing.T) *gorm.DB { return testDB(t) }
