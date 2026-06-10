package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

// Connect opens a GORM connection to PostgreSQL.
func Connect(databaseURL string) (*gorm.DB, error) {
	return ConnectWithPool(databaseURL, PoolConfig{
		MaxOpenConns:           20,
		MaxIdleConns:           5,
		ConnMaxLifetimeSeconds: 1800,
		ConnMaxIdleTimeSeconds: 300,
	})
}

// ConnectWithPool opens a GORM connection to PostgreSQL with explicit pool settings.
func ConnectWithPool(databaseURL string, pool PoolConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetimeSeconds) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTimeSeconds) * time.Second)

	slog.Info("connected to database",
		"max_open_conns", pool.MaxOpenConns,
		"max_idle_conns", pool.MaxIdleConns,
		"conn_max_lifetime_seconds", pool.ConnMaxLifetimeSeconds,
		"conn_max_idle_time_seconds", pool.ConnMaxIdleTimeSeconds,
	)
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
