package event_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestSystem_EmitAndQuery(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()
	sys, err := event.NewSystem(store)
	if err != nil {
		t.Fatalf("unexpected error creating System: %v", err)
	}

	// 1. Emit events
	e1, err := sys.Emit(
		ctx,
		event.TypeFileChanged,
		event.SourceWatchdog,
		"src/main.go",
		event.WithMetadata(map[string]any{"action": "write"}),
	)
	if err != nil {
		t.Fatalf("unexpected error emitting event: %v", err)
	}
	if e1 == nil || e1.ID == "" {
		t.Fatal("expected valid emitted event")
	}

	// 2. Query event
	retrieved, err := sys.Get(ctx, e1.ID)
	if err != nil {
		t.Fatalf("unexpected error getting event: %v", err)
	}
	if retrieved.Subject != "src/main.go" {
		t.Errorf("expected Subject src/main.go, got %s", retrieved.Subject)
	}

	// 3. Query by type
	byType, err := sys.EventsByType(ctx, event.TypeFileChanged)
	if err != nil || len(byType) != 1 {
		t.Fatalf("expected 1 event by type, got %d (err: %v)", len(byType), err)
	}

	// 4. Latest
	latest, err := sys.Latest(ctx, event.Query{Type: event.TypeFileChanged})
	if err != nil || latest == nil || latest.ID != e1.ID {
		t.Fatalf("expected latest event %s, got %+v (err: %v)", e1.ID, latest, err)
	}
}

func TestSystem_SnapshotAssociation(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()
	sys, _ := event.NewSystem(store)

	snapA := snapshot.ID("snap-aaa")
	snapB := snapshot.ID("snap-bbb")

	// Emit events associated with snapA
	_, _ = sys.Emit(ctx, event.TypeCommandExecuted, event.SourceUser, "make build", event.WithSnapshot(snapA))
	_, _ = sys.Emit(ctx, event.TypeSnapshotCreated, event.SourceRepro, string(snapA), event.WithSnapshot(snapA))

	// Emit event associated with snapB
	_, _ = sys.Emit(ctx, event.TypeSnapshotCreated, event.SourceRepro, string(snapB), event.WithSnapshot(snapB))

	// Emit unassociated event
	_, _ = sys.Emit(ctx, event.TypeCommandExecuted, event.SourceUser, "ls -la")

	// Verify events for snapA
	snapAEvents, err := sys.EventsForSnapshot(ctx, snapA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapAEvents) != 2 {
		t.Fatalf("expected 2 events for snapA, got %d", len(snapAEvents))
	}
	for _, e := range snapAEvents {
		if e.RelatedSnapshotID != snapA {
			t.Errorf("event %s has wrong snapshot association: %s", e.ID, e.RelatedSnapshotID)
		}
	}
}

// TestEventSystemIndependence verifies that events can be queried by time window
// and type without touching the Snapshot Store. This independence is what the
// Watchdog -> Event -> Snapshot Engine flow relies on.
func TestEventSystemIndependence(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// 1. Initialize persistent event store completely standalone (no snapshot store exists)
	fileStore, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating FileStore: %v", err)
	}
	sys, err := event.NewSystem(fileStore)
	if err != nil {
		t.Fatalf("unexpected error creating System: %v", err)
	}

	t0 := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	tWindowStart := t0.Add(5 * time.Minute)
	tWindowEnd := t0.Add(25 * time.Minute)

	// Emit a series of events across time without any snapshot store
	for i := 0; i < 6; i++ {
		evtTime := t0.Add(time.Duration(i*5) * time.Minute) // 0m, 5m, 10m, 15m, 20m, 25m
		tp := event.TypeFileChanged
		if i%2 == 1 {
			tp = event.TypeConfigChanged
		}

		_, err := sys.Emit(
			ctx,
			tp,
			event.SourceWatchdog,
			fmt.Sprintf("target_%d", i),
			event.WithTimestamp(evtTime),
		)
		if err != nil {
			t.Fatalf("failed emitting event %d: %v", i, err)
		}
	}

	// 2. Query strictly by time window (tWindowStart to tWindowEnd, which is 5m, 10m, 15m, 20m, 25m -> 5 events)
	windowEvents, err := sys.EventsInTimeWindow(ctx, tWindowStart, tWindowEnd, event.SortAsc)
	if err != nil {
		t.Fatalf("failed querying time window: %v", err)
	}
	if len(windowEvents) != 5 {
		t.Fatalf("expected 5 events in time window [%v, %v], got %d", tWindowStart, tWindowEnd, len(windowEvents))
	}

	// 3. Query strictly by time window AND type
	filtered, err := sys.Query(ctx, event.Query{
		After:  tWindowStart,
		Before: tWindowEnd,
		Type:   event.TypeFileChanged,
		Order:  event.SortAsc,
	})
	if err != nil {
		t.Fatalf("failed querying time window with type: %v", err)
	}
	// At 10m and 20m -> 2 events
	if len(filtered) != 2 {
		t.Fatalf("expected 2 TypeFileChanged events in window, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.Type != event.TypeFileChanged {
			t.Errorf("unexpected type: %s", e.Type)
		}
		if e.Timestamp.Before(tWindowStart) || e.Timestamp.After(tWindowEnd) {
			t.Errorf("event timestamp %v outside window [%v, %v]", e.Timestamp, tWindowStart, tWindowEnd)
		}
	}

	// Clean shutdown
	if err := sys.Close(); err != nil {
		t.Errorf("unexpected error closing System: %v", err)
	}
}
