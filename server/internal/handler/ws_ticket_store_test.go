package handler

import (
	"testing"
	"time"
)

func TestWSTicketStore_MintAndConsume(t *testing.T) {
	s := newWSTicketStore()
	token, err := s.Mint("user-1", "org-1", "claude")
	if err != nil {
		t.Fatalf("mint failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty ticket")
	}

	got, ok := s.Consume(token, "org-1")
	if !ok {
		t.Fatal("expected ticket to be valid")
	}
	if got.UserID != "user-1" || got.OrgID != "org-1" || got.Provider != "claude" {
		t.Errorf("unexpected ticket data: %+v", got)
	}
}

func TestWSTicketStore_SingleUse(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-1", "claude")

	if _, ok := s.Consume(token, "org-1"); !ok {
		t.Fatal("first consume should succeed")
	}
	if _, ok := s.Consume(token, "org-1"); ok {
		t.Fatal("second consume of same ticket must fail")
	}
}

func TestWSTicketStore_Expiry(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-1", "claude")

	// Force expiry by rewriting the stored entry's ExpiresAt into the past.
	s.mu.Lock()
	entry := s.tickets[token]
	entry.ExpiresAt = time.Now().Add(-1 * time.Second)
	s.tickets[token] = entry
	s.mu.Unlock()

	if _, ok := s.Consume(token, "org-1"); ok {
		t.Fatal("expired ticket must not be consumable")
	}
}

func TestWSTicketStore_OrgMismatch(t *testing.T) {
	s := newWSTicketStore()
	token, _ := s.Mint("user-1", "org-A", "claude")

	if _, ok := s.Consume(token, "org-B"); ok {
		t.Fatal("ticket minted for org-A must not validate against org-B")
	}
}
