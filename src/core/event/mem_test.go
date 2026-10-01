package event_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
)

func TestMemStore_RecordAndGet(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()

	evt, err := event.New(
		event.TypeFileChanged,
		event.SourceWatchdog,
		"main.go",
		event.WithID("evt-001"),
	)
	if err != nil {
		t.Fatalf("unexpected error creating event: %v", err)
	}

	// 1. Record event
	if err := store.Record(ctx, evt); err != nil {
		t.Fatalf("unexpected error recording event: %v", err)
	}

	// 2. Immutability: duplicate ID must fail
	if err := store.Record(ctx, evt); err == nil || !errors.Is(err, errors.CodeStorageFailure) {
		t.Errorf("expected CodeStorageFailure on duplicate record, got: %v", err)
	}

	// 3. Get existing
	got, err := store.Get(ctx, "evt-001")
	if err != nil {
		t.Fatalf("unexpected error getting event: %v", err)
	}
	if got.ID != "evt-001" || got.Subject != "main.go" {
		t.Errorf("unexpected event content: %+v", got)
	}

	// 4. Get non-existing
	if _, err := store.Get(ctx, "non-existent"); err == nil || !errors.Is(err, errors.CodeNotFound) {
		t.Errorf("expected CodeNotFound for missing event, got: %v", err)
	}

	// 5. Exists check
	exists, err := store.Exists(ctx, "evt-001")
	if err != nil || !exists {
		t.Errorf("expected exists=true, got %v, err=%v", exists, err)
	}
	exists, err = store.Exists(ctx, "non-existent")
	if err != nil || exists {
		t.Errorf("expected exists=false, got %v, err=%v", exists, err)
	}
}

func TestMemStore_QueryAndCount(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()

	baseTime := time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)
	for i := 1; i <= 5; i++ {
		evt, _ := event.New(
			event.TypeFileChanged,
			event.SourceWatchdog,
			fmt.Sprintf("file_%d.go", i),
			event.WithID(event.ID(fmt.Sprintf("evt-%03d", i))),
			event.WithTimestamp(baseTime.Add(time.Duration(i)*time.Minute)),
		)
		_ = store.Record(ctx, evt)
	}

	// Another type
	cmdEvt, _ := event.New(
		event.TypeCommandExecuted,
		event.SourceUser,
		"go test",
		event.WithID("evt-cmd-1"),
		event.WithTimestamp(baseTime.Add(10*time.Minute)),
	)
	_ = store.Record(ctx, cmdEvt)

	// Query by Type
	results, err := store.Query(ctx, event.Query{
		Type:  event.TypeFileChanged,
		Order: event.SortAsc,
	})
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	if results[0].ID != "evt-001" || results[4].ID != "evt-005" {
		t.Errorf("unexpected SortAsc order: %s to %s", results[0].ID, results[4].ID)
	}

	// Query with Limit and Offset
	paged, err := store.Query(ctx, event.Query{
		Type:   event.TypeFileChanged,
		Order:  event.SortAsc,
		Offset: 2,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("unexpected paged query error: %v", err)
	}
	if len(paged) != 2 || paged[0].ID != "evt-003" || paged[1].ID != "evt-004" {
		t.Errorf("unexpected paged results: %+v", paged)
	}

	// Count
	count, err := store.Count(ctx, event.Query{Type: event.TypeFileChanged})
	if err != nil || count != 5 {
		t.Errorf("expected count=5, got %d, err=%v", count, err)
	}
}

func TestMemStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()

	evt, _ := event.New(event.TypeGitCommit, event.SourceRepro, "initial", event.WithID("evt-del"))
	_ = store.Record(ctx, evt)

	if err := store.Delete(ctx, "evt-del"); err != nil {
		t.Fatalf("unexpected error deleting event: %v", err)
	}

	if err := store.Delete(ctx, "evt-del"); err == nil || !errors.Is(err, errors.CodeNotFound) {
		t.Errorf("expected CodeNotFound deleting non-existent event, got %v", err)
	}
}

func TestMemStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()

	const numWorkers = 10
	const numEvents = 20

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < numEvents; i++ {
				evt, err := event.New(
					event.TypeFileChanged,
					event.SourceWatchdog,
					fmt.Sprintf("w%d_file%d.go", workerID, i),
					event.WithID(event.ID(fmt.Sprintf("w%d-evt-%d", workerID, i))),
				)
				if err != nil {
					t.Errorf("error creating event: %v", err)
					return
				}
				if err := store.Record(ctx, evt); err != nil {
					t.Errorf("error recording event: %v", err)
					return
				}

				// Concurrent read
				_, _ = store.Query(ctx, event.Query{
					Source: event.SourceWatchdog,
					Limit:  5,
				})
			}
		}(w)
	}

	wg.Wait()

	total, err := store.Count(ctx, event.Query{})
	if err != nil {
		t.Fatalf("unexpected count error: %v", err)
	}
	expected := numWorkers * numEvents
	if total != expected {
		t.Fatalf("expected %d total events, got %d", expected, total)
	}
}
