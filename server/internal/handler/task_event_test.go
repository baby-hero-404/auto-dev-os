package handler

import (
	"context"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TestStreamEventsLoop_NoDuplicateOnRace reproduces design.md's reconnect
// race: an event committed to the DB (and broadcast to the live channel)
// between Subscribe and the catch-up query landing in *both* the catch-up
// history and the live buffer must be emitted exactly once, not twice.
func TestStreamEventsLoop_NoDuplicateOnRace(t *testing.T) {
	ch := make(chan models.TaskEvent, 4)
	raceEvent := models.TaskEvent{ID: "evt-race", SequenceNumber: 5}
	ch <- raceEvent // simulates the event landing in the live channel during catchup()

	var emitted []models.TaskEvent
	catchup := func() ([]models.TaskEvent, error) {
		// simulates the same event also being visible to the catch-up query,
		// since it was committed to the DB before the query ran
		return []models.TaskEvent{{ID: "evt-1", SequenceNumber: 4}, raceEvent}, nil
	}
	emit := func(e models.TaskEvent) { emitted = append(emitted, e) }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = streamEventsLoop(ctx, ch, catchup, emit)

	count := 0
	for _, e := range emitted {
		if e.SequenceNumber == 5 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("sequence_number 5 emitted %d times, want exactly 1 (emitted: %+v)", count, emitted)
	}
}
