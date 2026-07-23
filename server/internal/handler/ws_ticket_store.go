package handler

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// wsTicket is a single-use, short-lived, org-scoped credential exchanged
// for a query-param-safe value so the raw JWT never appears in a WS URL.
type wsTicket struct {
	UserID    string
	OrgID     string
	Provider  string
	ExpiresAt time.Time
}

const wsTicketTTL = 20 * time.Second

// wsTicketStore is an in-memory single-replica store. If the API ever runs
// multiple replicas without sticky WS routing, this must move to Redis —
// out of scope while the service is single-instance (see design.md Risk Mitigation).
type wsTicketStore struct {
	mu      sync.Mutex
	tickets map[string]wsTicket
}

func newWSTicketStore() *wsTicketStore {
	return &wsTicketStore{tickets: make(map[string]wsTicket)}
}

// Mint creates a new random ticket bound to the given identity and returns
// the opaque token to hand back to the client.
func (s *wsTicketStore) Mint(userID, orgID, provider string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()
	s.tickets[token] = wsTicket{
		UserID:    userID,
		OrgID:     orgID,
		Provider:  provider,
		ExpiresAt: time.Now().Add(wsTicketTTL),
	}
	return token, nil
}

// Consume validates and deletes a ticket in one atomic step (single-use).
// Returns ok=false if the ticket is missing, expired, or already consumed.
func (s *wsTicketStore) Consume(token, orgID string) (wsTicket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, found := s.tickets[token]
	delete(s.tickets, token) // single-use: always remove, valid or not

	if !found || time.Now().After(t.ExpiresAt) || t.OrgID != orgID {
		return wsTicket{}, false
	}
	return t, true
}

func (s *wsTicketStore) evictExpiredLocked() {
	now := time.Now()
	for k, v := range s.tickets {
		if now.After(v.ExpiresAt) {
			delete(s.tickets, k)
		}
	}
}
