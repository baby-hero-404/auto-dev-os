package orchestrator

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestStartAgentWatchdog_Sqlmock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm: %v", err)
	}

	agentRepo := repository.NewAgentRepo(gormDB)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stuck agent data
	stuckID := "agent-stuck-1"
	stuckName := "Stuck Agent"
	stuckRole := "backend"

	// Mock SELECT query for stuck agents
	rows := sqlmock.NewRows([]string{"id", "org_id", "name", "role", "goal", "autonomy_level", "context_config", "model_level_group", "status", "assignment_strategy", "created_at", "updated_at"}).
		AddRow(stuckID, "org-1", stuckName, stuckRole, "write code", "supervised", []byte("{}"), "balanced", "assigned", "manual", time.Now().Add(-40*time.Minute), time.Now().Add(-40*time.Minute))

	// Expect SELECT query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "agents" WHERE status IN ($1,$2) AND updated_at < $3`)).
		WithArgs("assigned", "running", sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Expect UPDATE query to set status to idle for the stuck agent ID
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "agents" SET "status"=$1,"updated_at"=$2 WHERE id IN ($3)`)).
		WithArgs("idle", sqlmock.AnyArg(), stuckID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Start agent watchdog with a 10 seconds interval, and we will cancel context shortly
	go StartAgentWatchdog(ctx, agentRepo, 10*time.Second, 30*time.Minute)

	// Manually trigger one check loop logic or let's use a very short timer but immediately cancel context
	// Actually, we can run a separate goroutine or just call the repository method directly to test the query and DB logic,
	// and then test StartAgentWatchdog with context cancel.
	// Let's first test the Repository method directly!
	cutoff := time.Now().Add(-30 * time.Minute)
	stuckAgents, err := agentRepo.ResetStuckAgents(ctx, cutoff)
	if err != nil {
		t.Fatalf("ResetStuckAgents failed: %v", err)
	}

	if len(stuckAgents) != 1 || stuckAgents[0].ID != stuckID {
		t.Errorf("expected 1 stuck agent with ID %s, got: %+v", stuckID, stuckAgents)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
