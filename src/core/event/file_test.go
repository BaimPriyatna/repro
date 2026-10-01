package event_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
)

func TestFileStore_Basics(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating FileStore: %v", err)
	}

	evt, err := event.New(
		event.TypeFileChanged,
		event.SourceWatchdog,
		"internal/config/config.go",
		event.WithID("evt-file-1"),
		event.WithSnapshot("snap-base"),
	)
	if err != nil {
		t.Fatalf("unexpected error creating event: %v", err)
	}

	// 1. Record
	if err := store.Record(ctx, evt); err != nil {
		t.Fatalf("unexpected error recording event: %v", err)
	}

	// Verify file exists on disk
	expectedPath := filepath.Join(tempDir, "events", "evt-file-1.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("event file not found on disk at %s: %v", expectedPath, err)
	}

	// 2. Immutability: duplicate record must fail
	if err := store.Record(ctx, evt); err == nil || !errors.Is(err, errors.CodeStorageFailure) {
		t.Errorf("expected CodeStorageFailure on duplicate disk write, got: %v", err)
	}

	// 3. Get
	got, err := store.Get(ctx, "evt-file-1")
	if err != nil {
		t.Fatalf("unexpected error getting event: %v", err)
	}
	if got.ID != "evt-file-1" || got.Subject != "internal/config/config.go" || got.RelatedSnapshotID != "snap-base" {
		t.Errorf("unexpected event loaded from disk: %+v", got)
	}

	// 4. Missing event
	if _, err := store.Get(ctx, "missing-id"); err == nil || !errors.Is(err, errors.CodeNotFound) {
		t.Errorf("expected CodeNotFound for missing event, got: %v", err)
	}

	// 5. Exists
	exists, err := store.Exists(ctx, "evt-file-1")
	if err != nil || !exists {
		t.Errorf("expected exists=true, got: %v, err=%v", exists, err)
	}
	exists, err = store.Exists(ctx, "missing-id")
	if err != nil || exists {
		t.Errorf("expected exists=false, got: %v, err=%v", exists, err)
	}
}

func TestFileStore_IndexRebuildOnRestart(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store1, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("error creating store1: %v", err)
	}

	// Record 3 events
	for i := 1; i <= 3; i++ {
		evt, _ := event.New(
			event.TypeGitCommit,
			event.SourceRepro,
			fmt.Sprintf("commit-%d", i),
			event.WithID(event.ID(fmt.Sprintf("evt-rebuild-%d", i))),
		)
		if err := store1.Record(ctx, evt); err != nil {
			t.Fatalf("failed recording event %d: %v", i, err)
		}
	}

	// Reopen store from same baseDir (simulating process restart)
	store2, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("error creating store2 from existing directory: %v", err)
	}

	count, err := store2.Count(ctx, event.Query{Type: event.TypeGitCommit})
	if err != nil || count != 3 {
		t.Fatalf("expected 3 events after index rebuild, got %d (err: %v)", count, err)
	}

	results, err := store2.Query(ctx, event.Query{Type: event.TypeGitCommit})
	if err != nil || len(results) != 3 {
		t.Fatalf("expected 3 queried events, got %d", len(results))
	}
}

func TestFileStore_CorruptionDetection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	evt, _ := event.New(event.TypeFileChanged, event.SourceWatchdog, "test.go", event.WithID("evt-corrupt"))
	if err := store.Record(ctx, evt); err != nil {
		t.Fatalf("failed recording event: %v", err)
	}

	// Corrupt the file on disk directly
	filePath := filepath.Join(tempDir, "events", "evt-corrupt.json")
	if err := os.WriteFile(filePath, []byte("NOT_VALID_JSON{{{"), 0o600); err != nil {
		t.Fatalf("failed writing corrupt content: %v", err)
	}

	// Load should detect silent corruption
	_, err = store.Get(ctx, "evt-corrupt")
	if err == nil || !errors.Is(err, errors.CodeStorageFailure) {
		t.Errorf("expected CodeStorageFailure on corrupt file, got: %v", err)
	}
}

func TestFileStore_PathTraversalRejection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	traversalIDs := []event.ID{
		"../escape",
		"../../etc/passwd",
		"sub/folder",
		"bad:id",
		"bad*id",
		".",
		"..",
	}

	for _, id := range traversalIDs {
		t.Run(string(id), func(t *testing.T) {
			_, err := store.Get(ctx, id)
			if err == nil || !errors.Is(err, errors.CodeInvalidInput) {
				t.Errorf("expected CodeInvalidInput for traversal ID %q, got: %v", id, err)
			}
		})
	}
}

func TestFileStore_Delete(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	evt, _ := event.New(event.TypeConfigChanged, event.SourceUser, "config.toml", event.WithID("evt-del-file"))
	_ = store.Record(ctx, evt)

	if err := store.Delete(ctx, "evt-del-file"); err != nil {
		t.Fatalf("unexpected error deleting event: %v", err)
	}

	// Verify file is gone from disk
	filePath := filepath.Join(tempDir, "events", "evt-del-file.json")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed from disk, stat err: %v", err)
	}

	// Deleting again should return CodeNotFound
	if err := store.Delete(ctx, "evt-del-file"); err == nil || !errors.Is(err, errors.CodeNotFound) {
		t.Errorf("expected CodeNotFound deleting missing event, got: %v", err)
	}
}

func TestFileStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := event.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	const numWorkers = 8
	const numEvents = 15

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
					event.WithID(event.ID(fmt.Sprintf("w%d-fevt-%d", workerID, i))),
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
					Limit:  3,
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
