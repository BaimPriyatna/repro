package diff_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestDiff_NilInputs(t *testing.T) {
	snap := &snapshot.Snapshot{
		ID: snapshot.NewID(),
	}

	_, err := diff.Diff(nil, snap)
	if err == nil {
		t.Error("Diff(nil, snap) should return error")
	}

	_, err = diff.Diff(snap, nil)
	if err == nil {
		t.Error("Diff(snap, nil) should return error")
	}
}

func TestDiff_IdenticalSnapshots(t *testing.T) {
	data := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
		"env": map[string]any{
			"NODE_ENV": "production",
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      data,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      data,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if result.HasChanges() {
		t.Error("identical snapshots should have no changes")
	}

	if result.Summary.Modified != 0 || result.Summary.Added != 0 || result.Summary.Removed != 0 {
		t.Errorf("summary = %+v, want no changes", result.Summary)
	}
}

func TestDiff_AddedField(t *testing.T) {
	dataOld := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	dataNew := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataOld,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataNew,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasChanges() {
		t.Error("should have detected changes")
	}

	if result.Summary.Added == 0 {
		t.Error("should have detected added fields")
	}

	// Check that runtime.node was added
	added := diff.FilterChanges(result.Changes, analysis.ChangeTypeAdded)
	found := false
	for _, change := range added {
		if change.Path == "runtime.node" {
			found = true
			break
		}
	}
	if !found {
		t.Error("did not detect runtime.node addition")
	}
}

func TestDiff_RemovedField(t *testing.T) {
	dataOld := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
	}

	dataNew := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataOld,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataNew,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasChanges() {
		t.Error("should have detected changes")
	}

	if result.Summary.Removed == 0 {
		t.Error("should have detected removed fields")
	}

	// Check that runtime.node was removed
	removed := diff.FilterChanges(result.Changes, analysis.ChangeTypeRemoved)
	found := false
	for _, change := range removed {
		if change.Path == "runtime.node" {
			found = true
			break
		}
	}
	if !found {
		t.Error("did not detect runtime.node removal")
	}
}

func TestDiff_ModifiedField(t *testing.T) {
	dataOld := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	dataNew := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.22.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataOld,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataNew,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasChanges() {
		t.Error("should have detected changes")
	}

	if result.Summary.Modified == 0 {
		t.Error("should have detected modified fields")
	}

	// Check that runtime.go.version was modified
	modified := diff.FilterChanges(result.Changes, analysis.ChangeTypeModified)
	found := false
	for _, change := range modified {
		if change.Path == "runtime.go.version" {
			found = true
			if change.OldValue != "1.21.0" {
				t.Errorf("OldValue = %v, want 1.21.0", change.OldValue)
			}
			if change.NewValue != "1.22.0" {
				t.Errorf("NewValue = %v, want 1.22.0", change.NewValue)
			}
			break
		}
	}
	if !found {
		t.Error("did not detect runtime.go.version modification")
	}
}

func TestDiff_ComplexChanges(t *testing.T) {
	dataOld := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
		"env": map[string]any{
			"NODE_ENV": "production",
		},
	}

	dataNew := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.22.0", // modified
			},
			"python": map[string]any{ // added
				"version": "3.11.0",
			},
			// node removed
		},
		"env": map[string]any{
			"NODE_ENV": "production", // unchanged
			"DEBUG":    "true",       // added
		},
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataOld,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataNew,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasChanges() {
		t.Error("should have detected changes")
	}

	// Should have added, removed, and modified
	if result.Summary.Added == 0 {
		t.Error("should have added fields")
	}
	if result.Summary.Removed == 0 {
		t.Error("should have removed fields")
	}
	if result.Summary.Modified == 0 {
		t.Error("should have modified fields")
	}
}

func TestDiff_SliceChanges(t *testing.T) {
	dataOld := map[string]any{
		"packages": []any{"pkg1", "pkg2", "pkg3"},
	}

	dataNew := map[string]any{
		"packages": []any{"pkg1", "pkg2", "pkg4"},
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataOld,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: dataNew,
	}

	result, err := diff.Diff(snapA, snapB)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasChanges() {
		t.Error("should have detected slice changes")
	}
}

func TestFilterChanges(t *testing.T) {
	changes := []analysis.Change{
		{Type: analysis.ChangeTypeAdded, Path: "a"},
		{Type: analysis.ChangeTypeRemoved, Path: "b"},
		{Type: analysis.ChangeTypeModified, Path: "c"},
		{Type: analysis.ChangeTypeUnchanged, Path: "d"},
	}

	added := diff.FilterChanges(changes, analysis.ChangeTypeAdded)
	if len(added) != 1 {
		t.Errorf("FilterChanges(Added) = %d, want 1", len(added))
	}

	removed := diff.FilterChanges(changes, analysis.ChangeTypeRemoved)
	if len(removed) != 1 {
		t.Errorf("FilterChanges(Removed) = %d, want 1", len(removed))
	}

	modified := diff.FilterChanges(changes, analysis.ChangeTypeModified)
	if len(modified) != 1 {
		t.Errorf("FilterChanges(Modified) = %d, want 1", len(modified))
	}

	addedOrRemoved := diff.FilterChanges(changes, analysis.ChangeTypeAdded, analysis.ChangeTypeRemoved)
	if len(addedOrRemoved) != 2 {
		t.Errorf("FilterChanges(Added, Removed) = %d, want 2", len(addedOrRemoved))
	}
}

func TestGetChangesByPrefix(t *testing.T) {
	changes := []analysis.Change{
		{Type: analysis.ChangeTypeModified, Path: "runtime.go.version"},
		{Type: analysis.ChangeTypeModified, Path: "runtime.node.version"},
		{Type: analysis.ChangeTypeModified, Path: "env.NODE_ENV"},
	}

	runtimeChanges := diff.GetChangesByPrefix(changes, "runtime")
	if len(runtimeChanges) != 2 {
		t.Errorf("GetChangesByPrefix(runtime) = %d, want 2", len(runtimeChanges))
	}

	envChanges := diff.GetChangesByPrefix(changes, "env")
	if len(envChanges) != 1 {
		t.Errorf("GetChangesByPrefix(env) = %d, want 1", len(envChanges))
	}

	noMatch := diff.GetChangesByPrefix(changes, "packages")
	if len(noMatch) != 0 {
		t.Errorf("GetChangesByPrefix(packages) = %d, want 0", len(noMatch))
	}
}

// Test deterministic ordering.
func TestDiff_DeterministicOrdering(t *testing.T) {
	data1 := map[string]any{
		"z": "value",
		"a": "value",
		"m": "value",
	}

	data2 := map[string]any{
		"z": "changed",
		"a": "changed",
		"m": "changed",
	}

	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: data1,
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: data2,
	}

	// Run diff multiple times
	result1, _ := diff.Diff(snapA, snapB)
	result2, _ := diff.Diff(snapA, snapB)

	// Changes should be in the same order
	if len(result1.Changes) != len(result2.Changes) {
		t.Fatal("inconsistent change count")
	}

	for i := range result1.Changes {
		if result1.Changes[i].Path != result2.Changes[i].Path {
			t.Errorf("non-deterministic ordering: run1[%d]=%s, run2[%d]=%s",
				i, result1.Changes[i].Path, i, result2.Changes[i].Path)
		}
	}
}
