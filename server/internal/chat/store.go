// Package chat wires the aicli conversation service into the API: a Postgres-backed
// conversation.Store (migration 000033), a Resolver that runs the CLI in a sandbox with an
// organization's stored provider credential, and the HTTP handler the router mounts.
package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/baby-hero-404/aicli/conversation"
	"github.com/baby-hero-404/aicli/run/stream"
	"gorm.io/gorm"
)

// Store keeps conversations, turns and events in Postgres (migration 000033). It implements
// conversation.Store and is safe for several API instances sharing one database: ClaimTurn and
// AppendEvents serialize on the conversation row.
type Store struct{ db *gorm.DB }

// NewStore returns a Store on db.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

var _ conversation.Store = (*Store)(nil)

type convRow struct {
	ID           string    `gorm:"column:id"`
	Owner        string    `gorm:"column:owner"`
	Profile      string    `gorm:"column:profile"`
	Title        string    `gorm:"column:title"`
	Resumable    bool      `gorm:"column:resumable"`
	SessionID    string    `gorm:"column:session_id"`
	ActiveTurnID string    `gorm:"column:active_turn_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

const convCols = `id, owner, profile, title, resumable, session_id, active_turn_id, created_at, updated_at`

func (r convRow) conversation() conversation.Conversation {
	return conversation.Conversation{
		ID: r.ID, Owner: conversation.Principal(r.Owner), Profile: r.Profile, Title: r.Title,
		Resumable: r.Resumable, SessionID: r.SessionID, ActiveTurnID: r.ActiveTurnID,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

type turnRow struct {
	ID                  string     `gorm:"column:id"`
	ConversationID      string     `gorm:"column:conversation_id"`
	ClientMessageID     string     `gorm:"column:client_message_id"`
	Status              string     `gorm:"column:status"`
	Failure             string     `gorm:"column:failure"`
	RetryAfterMS        int64      `gorm:"column:retry_after_ms"`
	CredentialsNotSaved bool       `gorm:"column:credentials_not_saved"`
	CancelRequested     bool       `gorm:"column:cancel_requested"`
	LeaseUntil          time.Time  `gorm:"column:lease_until"`
	StartedAt           time.Time  `gorm:"column:started_at"`
	FinishedAt          *time.Time `gorm:"column:finished_at"`
}

const turnCols = `id, conversation_id, client_message_id, status, failure, retry_after_ms,
	credentials_not_saved, cancel_requested, lease_until, started_at, finished_at`

func (r turnRow) turn() conversation.Turn {
	t := conversation.Turn{
		ID: r.ID, ConversationID: r.ConversationID, ClientMessageID: r.ClientMessageID,
		Status: conversation.TurnStatus(r.Status), Failure: conversation.FailureCode(r.Failure),
		RetryAfter:          time.Duration(r.RetryAfterMS) * time.Millisecond,
		CredentialsNotSaved: r.CredentialsNotSaved, CancelRequested: r.CancelRequested,
		LeaseUntil: r.LeaseUntil.UTC(), StartedAt: r.StartedAt.UTC(),
	}
	if r.FinishedAt != nil {
		t.FinishedAt = r.FinishedAt.UTC()
	}
	return t
}

type eventRow struct {
	ConversationID string    `gorm:"column:conversation_id"`
	Seq            int64     `gorm:"column:seq"`
	TurnID         string    `gorm:"column:turn_id"`
	At             time.Time `gorm:"column:at"`
	Kind           string    `gorm:"column:kind"`
	Body           string    `gorm:"column:body"`
	ToolName       *string   `gorm:"column:tool_name"`
	ToolSummary    *string   `gorm:"column:tool_summary"`
	ToolIsError    *bool     `gorm:"column:tool_is_error"`
	Meta           []byte    `gorm:"column:meta"`
}

const eventCols = `conversation_id, seq, turn_id, at, kind, body, tool_name, tool_summary, tool_is_error, meta`

func (r eventRow) event() (conversation.Event, error) {
	e := conversation.Event{
		ConversationID: r.ConversationID, Seq: r.Seq, TurnID: r.TurnID, Time: r.At.UTC(),
		Kind: stream.Kind(r.Kind), Text: r.Body,
	}
	if r.ToolName != nil {
		t := &stream.Tool{Name: *r.ToolName}
		if r.ToolSummary != nil {
			t.Summary = *r.ToolSummary
		}
		if r.ToolIsError != nil {
			t.IsError = *r.ToolIsError
		}
		e.Tool = t
	}
	if len(r.Meta) > 0 && string(r.Meta) != "null" {
		if err := json.Unmarshal(r.Meta, &e.Meta); err != nil {
			return conversation.Event{}, fmt.Errorf("chat: event %d meta: %w", r.Seq, err)
		}
	}
	return e, nil
}

func (s *Store) conn(ctx context.Context) *gorm.DB { return s.db.WithContext(ctx) }

// CreateConversation implements conversation.Store.
func (s *Store) CreateConversation(ctx context.Context, c conversation.Conversation) error {
	err := s.conn(ctx).Exec(`INSERT INTO chat_conversations (`+convCols+`) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		c.ID, string(c.Owner), c.Profile, c.Title, c.Resumable, c.SessionID, c.CreatedAt.UTC(), c.UpdatedAt.UTC()).Error
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return conversation.ErrInvalidArgument
		}
		return fmt.Errorf("chat: create conversation: %w", err)
	}
	return nil
}

// GetConversation implements conversation.Store.
func (s *Store) GetConversation(ctx context.Context, id string) (conversation.Conversation, error) {
	var row convRow
	res := s.conn(ctx).Raw(`SELECT `+convCols+` FROM chat_conversations WHERE id = ?`, id).Scan(&row)
	if res.Error != nil {
		return conversation.Conversation{}, fmt.Errorf("chat: get conversation: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return conversation.Conversation{}, conversation.ErrNotFound
	}
	return row.conversation(), nil
}

// ListConversations implements conversation.Store. The cursor encodes the last row's
// (created_at, id), so a page never repeats or skips a row when new ones arrive.
func (s *Store) ListConversations(ctx context.Context, owner conversation.Principal, cursor string, limit int) ([]conversation.Conversation, string, error) {
	var rows []convRow
	q := s.conn(ctx)
	var res *gorm.DB
	if cursor == "" {
		res = q.Raw(`SELECT `+convCols+` FROM chat_conversations WHERE owner = ?
			ORDER BY created_at DESC, id DESC LIMIT ?`, string(owner), limit+1).Scan(&rows)
	} else {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		res = q.Raw(`SELECT `+convCols+` FROM chat_conversations WHERE owner = ? AND (created_at, id) < (?, ?)
			ORDER BY created_at DESC, id DESC LIMIT ?`, string(owner), at, id, limit+1).Scan(&rows)
	}
	if res.Error != nil {
		return nil, "", fmt.Errorf("chat: list conversations: %w", res.Error)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCursor(last.CreatedAt, last.ID)
	}
	out := make([]conversation.Conversation, len(rows))
	for i, r := range rows {
		out[i] = r.conversation()
	}
	return out, next, nil
}

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMicro(), 10) + "|" + id))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", conversation.ErrInvalidCursor
	}
	micros, id, ok := strings.Cut(string(raw), "|")
	n, perr := strconv.ParseInt(micros, 10, 64)
	if !ok || perr != nil || id == "" {
		return time.Time{}, "", conversation.ErrInvalidCursor
	}
	return time.UnixMicro(n).UTC(), id, nil
}

// ClaimTurn implements conversation.Store. The conversation row is locked for the check and the
// insert, which is what makes "one running turn per conversation" hold across instances.
func (s *Store) ClaimTurn(ctx context.Context, t conversation.Turn) (conversation.Turn, error) {
	var out conversation.Turn
	err := s.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var c convRow
		res := tx.Raw(`SELECT `+convCols+` FROM chat_conversations WHERE id = ? FOR UPDATE`, t.ConversationID).Scan(&c)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return conversation.ErrNotFound
		}
		var existing turnRow
		res = tx.Raw(`SELECT `+turnCols+` FROM chat_turns WHERE conversation_id = ? AND client_message_id = ?`,
			t.ConversationID, t.ClientMessageID).Scan(&existing)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			out = existing.turn()
			return conversation.ErrDuplicate
		}
		if c.ActiveTurnID != "" {
			return conversation.ErrTurnInProgress
		}
		if err := tx.Exec(`INSERT INTO chat_turns (id, conversation_id, client_message_id, status, lease_until, started_at)
			VALUES (?, ?, ?, 'running', ?, ?)`, t.ID, t.ConversationID, t.ClientMessageID, t.LeaseUntil.UTC(), t.StartedAt.UTC()).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE chat_conversations SET active_turn_id = ? WHERE id = ?`, t.ID, t.ConversationID).Error; err != nil {
			return err
		}
		t.Status = conversation.TurnRunning
		out = t
		return nil
	})
	switch {
	case errors.Is(err, conversation.ErrDuplicate), errors.Is(err, conversation.ErrNotFound), errors.Is(err, conversation.ErrTurnInProgress):
		return out, err
	case err != nil:
		return conversation.Turn{}, fmt.Errorf("chat: claim turn: %w", err)
	}
	return out, nil
}

// GetTurn implements conversation.Store.
func (s *Store) GetTurn(ctx context.Context, id string) (conversation.Turn, error) {
	var row turnRow
	res := s.conn(ctx).Raw(`SELECT `+turnCols+` FROM chat_turns WHERE id = ?`, id).Scan(&row)
	if res.Error != nil {
		return conversation.Turn{}, fmt.Errorf("chat: get turn: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return conversation.Turn{}, conversation.ErrNotFound
	}
	return row.turn(), nil
}

// RenewLease implements conversation.Store.
func (s *Store) RenewLease(ctx context.Context, turnID string, until time.Time) error {
	return s.updateRunning(ctx, `UPDATE chat_turns SET lease_until = ? WHERE id = ? AND status = 'running'`, until.UTC(), turnID)
}

// RequestCancel implements conversation.Store.
func (s *Store) RequestCancel(ctx context.Context, turnID string) error {
	return s.updateRunning(ctx, `UPDATE chat_turns SET cancel_requested = true WHERE id = ? AND status = 'running'`, turnID)
}

// updateRunning runs an UPDATE restricted to running turns whose last argument is the turn ID:
// no row changed means the turn is finished (ErrTurnNotRunning) or unknown (ErrNotFound).
func (s *Store) updateRunning(ctx context.Context, query string, args ...any) error {
	res := s.conn(ctx).Exec(query, args...)
	if res.Error != nil {
		return fmt.Errorf("chat: update turn: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return nil
	}
	turnID, _ := args[len(args)-1].(string)
	if _, err := s.GetTurn(ctx, turnID); err != nil {
		return err
	}
	return conversation.ErrTurnNotRunning
}

// CancelRequested implements conversation.Store.
func (s *Store) CancelRequested(ctx context.Context, turnIDs []string) ([]string, error) {
	if len(turnIDs) == 0 {
		return nil, nil
	}
	var ids []string
	if err := s.conn(ctx).Raw(`SELECT id FROM chat_turns WHERE id IN ? AND cancel_requested AND status = 'running'`, turnIDs).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("chat: cancel requested: %w", err)
	}
	return ids, nil
}

// FinishTurn implements conversation.Store.
func (s *Store) FinishTurn(ctx context.Context, t conversation.Turn, sessionID string, resumable bool) error {
	err := s.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var convID string
		var finished *time.Time
		if !t.FinishedAt.IsZero() {
			f := t.FinishedAt.UTC()
			finished = &f
		}
		res := tx.Raw(`UPDATE chat_turns SET status = ?, failure = ?, retry_after_ms = ?, credentials_not_saved = ?, finished_at = ?
			WHERE id = ? RETURNING conversation_id`,
			string(t.Status), string(t.Failure), t.RetryAfter.Milliseconds(), t.CredentialsNotSaved, finished, t.ID).Scan(&convID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return conversation.ErrNotFound
		}
		updated := t.FinishedAt.UTC()
		if t.FinishedAt.IsZero() {
			updated = time.Now().UTC()
		}
		return tx.Exec(`UPDATE chat_conversations SET
				active_turn_id = CASE WHEN active_turn_id = ? THEN '' ELSE active_turn_id END,
				resumable = ?, session_id = CASE WHEN ? <> '' THEN ? ELSE session_id END, updated_at = ?
			WHERE id = ?`, t.ID, resumable, sessionID, sessionID, updated, convID).Error
	})
	if errors.Is(err, conversation.ErrNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("chat: finish turn: %w", err)
	}
	return nil
}

// StaleTurns implements conversation.Store.
func (s *Store) StaleTurns(ctx context.Context, now time.Time) ([]conversation.Turn, error) {
	var rows []turnRow
	if err := s.conn(ctx).Raw(`SELECT `+turnCols+` FROM chat_turns WHERE status = 'running' AND lease_until < ? ORDER BY id`, now.UTC()).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("chat: stale turns: %w", err)
	}
	out := make([]conversation.Turn, len(rows))
	for i, r := range rows {
		out[i] = r.turn()
	}
	return out, nil
}

// AppendEvents implements conversation.Store. Appends to one conversation run one at a time (the
// conversation row is locked), so Seq is consecutive with no gap even under concurrency.
func (s *Store) AppendEvents(ctx context.Context, conversationID string, events []conversation.Event) ([]conversation.Event, error) {
	out := make([]conversation.Event, len(events))
	err := s.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var locked string
		res := tx.Raw(`SELECT id FROM chat_conversations WHERE id = ? FOR UPDATE`, conversationID).Scan(&locked)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return conversation.ErrNotFound
		}
		var last int64
		if err := tx.Raw(`SELECT COALESCE(MAX(seq), 0) FROM chat_events WHERE conversation_id = ?`, conversationID).Scan(&last).Error; err != nil {
			return err
		}
		for i, e := range events {
			last++
			var meta any
			if len(e.Meta) > 0 {
				b, err := json.Marshal(e.Meta)
				if err != nil {
					return err
				}
				meta = string(b)
			}
			var name, summary *string
			var isErr *bool
			if e.Tool != nil {
				name, summary, isErr = &e.Tool.Name, &e.Tool.Summary, &e.Tool.IsError
			}
			if err := tx.Exec(`INSERT INTO chat_events (`+eventCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb)`,
				conversationID, last, e.TurnID, e.Time.UTC(), string(e.Kind), e.Text, name, summary, isErr, meta).Error; err != nil {
				return err
			}
			stored := cloneEvent(e)
			stored.ConversationID, stored.Seq = conversationID, last
			out[i] = stored
		}
		return nil
	})
	if errors.Is(err, conversation.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("chat: append events: %w", err)
	}
	return out, nil
}

// ListEvents implements conversation.Store.
func (s *Store) ListEvents(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]conversation.Event, error) {
	var exists int
	res := s.conn(ctx).Raw(`SELECT 1 FROM chat_conversations WHERE id = ?`, conversationID).Scan(&exists)
	if res.Error != nil {
		return nil, fmt.Errorf("chat: list events: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, conversation.ErrNotFound
	}
	if afterSeq < 0 {
		afterSeq = 0
	}
	var rows []eventRow
	if err := s.conn(ctx).Raw(`SELECT `+eventCols+` FROM chat_events WHERE conversation_id = ? AND seq > ? ORDER BY seq LIMIT ?`,
		conversationID, afterSeq, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("chat: list events: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]conversation.Event, len(rows))
	for i, r := range rows {
		e, err := r.event()
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

func cloneEvent(e conversation.Event) conversation.Event {
	if e.Tool != nil {
		t := *e.Tool
		e.Tool = &t
	}
	if e.Meta != nil {
		m := make(map[string]string, len(e.Meta))
		for k, v := range e.Meta {
			m[k] = v
		}
		e.Meta = m
	}
	return e
}
