package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gcdb "github.com/baby-hero-404/gocore/database"
	"github.com/baby-hero-404/gocore/database/postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

type PoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

// ConnectWithPool opens a GORM connection to PostgreSQL via gocore, with explicit pool settings,
// a startup ping and bounded connection retries.
func ConnectWithPool(databaseURL string, pool PoolConfig) (*gorm.DB, error) {
	db, err := postgres.Open(context.Background(), gcdb.Config{
		DSN:             databaseURL,
		MaxOpenConns:    pool.MaxOpenConns,
		MaxIdleConns:    pool.MaxIdleConns,
		ConnMaxLifetime: time.Duration(pool.ConnMaxLifetimeSeconds) * time.Second,
		ConnMaxIdleTime: time.Duration(pool.ConnMaxIdleTimeSeconds) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

// Migrate runs all pending migrations from the given directory.
func Migrate(databaseURL, migrationsPath string) error {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		slog.Info("migrations applied successfully")
	} else {
		slog.Info("migrations applied successfully", "version", version, "dirty", dirty)
	}
	return nil
}
