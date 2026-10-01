package store_test

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreerrors "github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

func makeTestSnapshot(id snapshot.ID, source snapshot.Source, parentID snapshot.ID, t time.Time, data map[string]any) *snapshot.Snapshot {
	hash, _ := hasher.ComputeContentHash(data)
	return &snapshot.Snapshot{
		ID:            id,
		Timestamp:     t,
		SchemaVersion: snapshot.SchemaVersion,
		ParentID:      parentID,
		ContentHash:   hash,
		Source:        source,
		Data:          data,
		Labels:        map[string]string{"env": "test"},
	}
}

func testStoreImplementation(t *testing.T, s store.Store) {
	ctx := context.Background()
	now := time.Now()

	s1 := makeTestSnapshot("snap-1", snapshot.SourceManual, "", now.Add(-2*time.Hour), map[string]any{"os": "linux"})
	s2 := makeTestSnapshot("snap-2", snapshot.SourceRepro, "snap-1", now.Add(-1*time.Hour), map[string]any{"os": "linux", "runtime": "go1.27"})
	s3 := makeTestSnapshot("snap-3", snapshot.SourceWatchdog, "snap-2", now, map[string]any{"os": "linux", "runtime": "go1.27", "dirty": true})

	// 1. Store
	if err := s.Store(ctx, s1); err != nil {
		t.Fatalf("Store(s1) failed: %v", err)
	}
	if err := s.Store(ctx, s2); err != nil {
		t.Fatalf("Store(s2) failed: %v", err)
	}
	if err := s.Store(ctx, s3); err != nil {
		t.Fatalf("Store(s3) failed: %v", err)
	}

	// 2. Immutability — Cannot overwrite
	err := s.Store(ctx, s1)
	if err == nil {
		t.Fatalf("expected error when storing duplicate snapshot ID (immutability rule)")
	}

	// 3. Exists
	exists, err := s.Exists(ctx, "snap-1")
	if err != nil || !exists {
		t.Errorf("expected exists=true for snap-1, got exists=%v, err=%v", exists, err)
	}
	exists, err = s.Exists(ctx, "non-existent")
	if err != nil || exists {
		t.Errorf("expected exists=false for non-existent, got exists=%v, err=%v", exists, err)
	}

	// 4. Load
	loaded, err := s.Load(ctx, "snap-2")
	if err != nil {
		t.Fatalf("Load(snap-2) failed: %v", err)
	}
	if loaded.ID != "snap-2" || loaded.ParentID != "snap-1" || loaded.Source != snapshot.SourceRepro {
		t.Errorf("loaded snapshot metadata mismatch: %+v", loaded)
	}
	if loaded.ContentHash != s2.ContentHash {
		t.Errorf("expected hash %q, got %q", s2.ContentHash, loaded.ContentHash)
	}

	// 5. Load non-existent
	_, err = s.Load(ctx, "missing-id")
	if err == nil {
		t.Fatalf("expected error loading missing snapshot")
	}
	var rErr *coreerrors.ReproError
	if !stderrors.As(err, &rErr) || rErr.Code != coreerrors.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}

	// 6. List with filtering and sorting
	list, err := s.List(ctx, store.Filter{})
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 snapshots in list, got %d", len(list))
	}
	// Verify descending timestamp order (newest first: snap-3, snap-2, snap-1)
	if list[0].ID != "snap-3" || list[1].ID != "snap-2" || list[2].ID != "snap-1" {
		t.Errorf("expected newest first order, got [%s, %s, %s]", list[0].ID, list[1].ID, list[2].ID)
	}

	// List filter by source
	listRepro, err := s.List(ctx, store.Filter{Source: snapshot.SourceRepro})
	if err != nil || len(listRepro) != 1 || listRepro[0].ID != "snap-2" {
		t.Errorf("filter by source failed: got %d items", len(listRepro))
	}

	// List filter by parent
	listParent, err := s.List(ctx, store.Filter{ParentID: "snap-1"})
	if err != nil || len(listParent) != 1 || listParent[0].ID != "snap-2" {
		t.Errorf("filter by parent failed: got %d items", len(listParent))
	}

	// List with limit
	listLimited, err := s.List(ctx, store.Filter{Limit: 2})
	if err != nil || len(listLimited) != 2 {
		t.Errorf("filter with limit 2 failed: got %d items", len(listLimited))
	}

	// 7. ListMetadata
	metas, err := s.ListMetadata(ctx, store.Filter{})
	if err != nil {
		t.Fatalf("ListMetadata failed: %v", err)
	}
	if len(metas) != 3 {
		t.Fatalf("expected 3 metas, got %d", len(metas))
	}
	if metas[0].ID != "snap-3" {
		t.Errorf("expected snap-3 first in metadata list, got %s", metas[0].ID)
	}

	// 8. Delete
	if err := s.Delete(ctx, "snap-1"); err != nil {
		t.Fatalf("Delete(snap-1) failed: %v", err)
	}
	exists, _ = s.Exists(ctx, "snap-1")
	if exists {
		t.Errorf("snap-1 should not exist after delete")
	}

	err = s.Delete(ctx, "snap-1")
	if err == nil {
		t.Errorf("expected error deleting already deleted snapshot")
	}
}

func TestMemStore(t *testing.T) {
	s := store.NewMemStore()
	testStoreImplementation(t, s)
}

func TestMemStore_CorruptionDetection(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemStore()

	snap := makeTestSnapshot("corrupted-1", snapshot.SourceManual, "", time.Now(), map[string]any{"key": "original"})
	snap.ContentHash = "tampered_hash_value"

	// Bypassing normal checks when storing to test load corruption detection
	_ = s.Store(ctx, snap)

	_, err := s.Load(ctx, "corrupted-1")
	if err == nil {
		t.Fatalf("expected error loading corrupted snapshot")
	}
	var rErr *coreerrors.ReproError
	if !stderrors.As(err, &rErr) || rErr.Code != coreerrors.CodeStorageFailure {
		t.Errorf("expected CodeStorageFailure, got %v", err)
	}
}

func TestFileStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repro_filestore_*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs, err := store.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	testStoreImplementation(t, fs)
}

func TestFileStore_CorruptionDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repro_filestore_corrupt_*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs, err := store.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()
	snap := makeTestSnapshot("snap-corrupt", snapshot.SourceManual, "", time.Now(), map[string]any{"data": "valid"})
	if err := fs.Store(ctx, snap); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Intentionally corrupt the file on disk by modifying payload
	filePath := filepath.Join(tempDir, "snapshots", "snap-corrupt.json")
	corruptedContent := []byte(`{"id":"snap-corrupt","timestamp":"2026-09-28T00:00:00Z","schema_version":1,"content_hash":"validhash","source":"manual","data":{"data":"TAMPERED"}}`)
	if err := os.WriteFile(filePath, corruptedContent, 0o600); err != nil {
		t.Fatalf("writing corrupt content: %v", err)
	}

	_, err = fs.Load(ctx, "snap-corrupt")
	if err == nil {
		t.Fatalf("expected corruption detection error on Load")
	}
	var rErr *coreerrors.ReproError
	if !stderrors.As(err, &rErr) || rErr.Code != coreerrors.CodeStorageFailure {
		t.Errorf("expected CodeStorageFailure for corrupted file, got: %v", err)
	}
}
