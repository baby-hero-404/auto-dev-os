package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/baby-hero-404/aicli/conversation"
	"github.com/baby-hero-404/aicli/conversation/storetest"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDB returns a gorm handle on a fresh schema with migration 000033 applied. It skips the test
// when TEST_DATABASE_URL (a throwaway Postgres) is not set.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a throwaway Postgres to run the store contract suite")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := admin.DB()
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema)).Error
		_ = sqlDB.Close()
	})

	scoped := dsn + " search_path=" + schema // key=value DSN
	if strings.Contains(dsn, "://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		scoped = dsn + sep + "search_path=" + schema // URL DSN
	}
	db, err := gorm.Open(postgres.Open(scoped), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("connect to schema: %v", err)
	}
	up, err := os.ReadFile(filepath.Join("..", "..", "migration", "000033_add_chat_conversations.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(up)).Error; err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	inner, _ := db.DB()
	t.Cleanup(func() { _ = inner.Close() })
	return db
}

// The Postgres store must pass the same contract suite as aicli's reference store.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) conversation.Store { return NewStore(testDB(t)) })
}
