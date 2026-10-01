package engine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/collector"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

// mockCollector implements collector.Collector for deterministic test state.
type mockCollector struct {
	name string
	data any
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(_ context.Context) (collector.Result, error) {
	return collector.Result{Data: m.data}, nil
}

func TestEngine_Initialization(t *testing.T) {
	_, err := engine.New(nil)
	if err == nil {
		t.Fatalf("expected error creating engine with nil store")
	}

	mem := store.NewMemStore()
	eng, err := engine.New(mem)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	// Register custom collector
	err = eng.RegisterCollector(&mockCollector{name: "custom", data: "custom_value"})
	if err != nil {
		t.Fatalf("unexpected error registering custom collector: %v", err)
	}
}

func TestEngine_CaptureAndVerifyExitCriteria(t *testing.T) {
	// : a snapshot can be created, hashed, stored, reloaded byte-identical,
	// and diffed against itself with zero drift.
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "repro_engine_exit_criteria_*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs, err := store.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("creating FileStore: %v", err)
	}

	eng, err := engine.New(fs)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	// 1. Capture snapshot
	snap, err := eng.Capture(ctx, snapshot.SourceManual,
		engine.WithParent("parent-123"),
		engine.WithLabels(map[string]string{"env": "prod", "cluster": "us-east"}),
	)
	if err != nil {
		t.Fatalf("Capture() failed: %v", err)
	}

	// Verify required properties
	if snap.ID == "" || len(snap.ID) != 36 {
		t.Errorf("expected valid UUID v4 ID, got %q", snap.ID)
	}
	if snap.SchemaVersion != snapshot.SchemaVersion {
		t.Errorf("expected schema version %d, got %d", snapshot.SchemaVersion, snap.SchemaVersion)
	}
	if snap.ParentID != "parent-123" {
		t.Errorf("expected parent ID 'parent-123', got %q", snap.ParentID)
	}
	if snap.Source != snapshot.SourceManual {
		t.Errorf("expected source 'manual', got %q", snap.Source)
	}
	if snap.ContentHash == "" {
		t.Fatalf("expected non-empty content hash")
	}
	if snap.Labels["env"] != "prod" {
		t.Errorf("expected label 'env'='prod', got %q", snap.Labels["env"])
	}

	// 2. Verify Content Hash calculation
	valid, err := hasher.VerifyContentHash(snap.Data, snap.ContentHash)
	if err != nil || !valid {
		t.Errorf("content hash verification failed: valid=%v, err=%v", valid, err)
	}

	// 3. Store snapshot to FileStore
	if err := eng.Store(ctx, snap); err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// 4. Reload from store byte-identical
	reloaded, err := eng.Load(ctx, snap.ID)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if reloaded.ID != snap.ID {
		t.Errorf("ID mismatch: got %q, want %q", reloaded.ID, snap.ID)
	}
	if reloaded.ContentHash != snap.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", reloaded.ContentHash, snap.ContentHash)
	}
	if reloaded.ParentID != snap.ParentID {
		t.Errorf("ParentID mismatch: got %q, want %q", reloaded.ParentID, snap.ParentID)
	}

	// 5. Compare against itself — verify ZERO DRIFT
	compSelf, err := eng.Compare(snap, reloaded)
	if err != nil {
		t.Fatalf("Compare() failed: %v", err)
	}

	if !compSelf.Identical {
		t.Errorf("expected Identical=true when diffed against itself (zero drift)")
	}
	if len(compSelf.ModifiedKeys) != 0 || len(compSelf.AddedKeys) != 0 || len(compSelf.RemovedKeys) != 0 {
		t.Errorf("expected zero modified/added/removed keys, got: mod=%v, add=%v, rem=%v",
			compSelf.ModifiedKeys, compSelf.AddedKeys, compSelf.RemovedKeys)
	}
}

func TestEngine_CaptureCollectorSubset(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemStore()
	eng, err := engine.New(mem)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	snap, err := eng.Capture(ctx, snapshot.SourceRepro, engine.WithCollectors("os", "runtime"))
	if err != nil {
		t.Fatalf("Capture() failed: %v", err)
	}

	if len(snap.Data) != 2 {
		t.Fatalf("expected exactly 2 collectors in data map, got %d", len(snap.Data))
	}
	if _, ok := snap.Data["os"]; !ok {
		t.Errorf("expected 'os' collector in data")
	}
	if _, ok := snap.Data["runtime"]; !ok {
		t.Errorf("expected 'runtime' collector in data")
	}
	if _, ok := snap.Data["git"]; ok {
		t.Errorf("expected 'git' collector NOT in data")
	}
}

func TestEngine_CompareWithDifferences(t *testing.T) {
	s1 := &snapshot.Snapshot{
		ID:        "snap-base",
		Timestamp: time.Now(),
		Data: map[string]any{
			"os":      "linux",
			"runtime": "go1.26",
			"deleted": "will_be_removed",
		},
	}
	s1.ContentHash, _ = hasher.ComputeContentHash(s1.Data)

	s2 := &snapshot.Snapshot{
		ID:        "snap-target",
		Timestamp: time.Now().Add(time.Minute),
		Data: map[string]any{
			"os":      "linux",
			"runtime": "go1.27",
			"added":   "new_key",
		},
	}
	s2.ContentHash, _ = hasher.ComputeContentHash(s2.Data)

	comp, err := engine.Compare(s1, s2)
	if err != nil {
		t.Fatalf("Compare() failed: %v", err)
	}

	if comp.Identical {
		t.Errorf("expected Identical=false for different snapshots")
	}

	// Added
	if len(comp.AddedKeys) != 1 || comp.AddedKeys[0] != "added" {
		t.Errorf("expected ['added'], got %v", comp.AddedKeys)
	}

	// Removed
	if len(comp.RemovedKeys) != 1 || comp.RemovedKeys[0] != "deleted" {
		t.Errorf("expected ['deleted'], got %v", comp.RemovedKeys)
	}

	// Modified
	if len(comp.ModifiedKeys) != 1 || comp.ModifiedKeys[0] != "runtime" {
		t.Errorf("expected ['runtime'], got %v", comp.ModifiedKeys)
	}

	// Unchanged
	if len(comp.UnchangedKeys) != 1 || comp.UnchangedKeys[0] != "os" {
		t.Errorf("expected ['os'], got %v", comp.UnchangedKeys)
	}
}

func TestEngine_StoreTamperedSnapshotFails(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemStore()
	eng, err := engine.New(mem)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	snap := &snapshot.Snapshot{
		ID:          "tampered-snap",
		ContentHash: "invalid_hash_string",
		Source:      snapshot.SourceManual,
		Data:        map[string]any{"os": "linux"},
	}

	err = eng.Store(ctx, snap)
	if err == nil {
		t.Fatalf("expected Store to reject snapshot with mismatched content hash")
	}
}
