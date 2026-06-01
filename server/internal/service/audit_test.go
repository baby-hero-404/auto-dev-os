package service

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestAuditOption_Compose(t *testing.T) {
	entry := models.AuditLog{}
	orgID := "org-1"
	userID := "user-2"
	opts := []AuditOption{
		WithOrgID(orgID),
		WithUserID(userID),
		WithIPAddress("10.0.0.1"),
		WithDetails(map[string]int{"count": 5}),
	}
	for _, opt := range opts {
		opt(&entry)
	}
	if entry.OrgID == nil || *entry.OrgID != orgID {
		t.Error("OrgID not set")
	}
	if entry.UserID == nil || *entry.UserID != userID {
		t.Error("UserID not set")
	}
	if entry.IPAddress != "10.0.0.1" {
		t.Error("IPAddress not set")
	}
	if string(entry.Details) != `{"count":5}` {
		t.Errorf("Details mismatch: %s", string(entry.Details))
	}
}

func TestAuditOption_WithDetails(t *testing.T) {
	entry := models.AuditLog{}
	opt := WithDetails(map[string]string{"key": "value"})
	opt(&entry)

	if string(entry.Details) != `{"key":"value"}` {
		t.Errorf("expected details to be set, got: %s", string(entry.Details))
	}
}

func TestAuditOption_WithOrgID(t *testing.T) {
	entry := models.AuditLog{}
	id := "org-123"
	WithOrgID(id)(&entry)
	if entry.OrgID == nil || *entry.OrgID != id {
		t.Error("expected OrgID to be set")
	}
}

func TestAuditOption_WithUserID(t *testing.T) {
	entry := models.AuditLog{}
	id := "user-456"
	WithUserID(id)(&entry)
	if entry.UserID == nil || *entry.UserID != id {
		t.Error("expected UserID to be set")
	}
}

func TestAuditOption_WithAgentID(t *testing.T) {
	entry := models.AuditLog{}
	id := "agent-789"
	WithAgentID(id)(&entry)
	if entry.AgentID == nil || *entry.AgentID != id {
		t.Error("expected AgentID to be set")
	}
}

func TestAuditOption_WithTaskID(t *testing.T) {
	entry := models.AuditLog{}
	id := "task-000"
	WithTaskID(id)(&entry)
	if entry.TaskID == nil || *entry.TaskID != id {
		t.Error("expected TaskID to be set")
	}
}

func TestAuditOption_WithIPAddress(t *testing.T) {
	entry := models.AuditLog{}
	ip := "192.168.1.1"
	WithIPAddress(ip)(&entry)
	if entry.IPAddress != ip {
		t.Error("expected IPAddress to be set")
	}
}
