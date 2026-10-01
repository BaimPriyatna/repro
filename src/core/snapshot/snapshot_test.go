package snapshot_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestSnapshot_IsEmpty(t *testing.T) {
	t.Run("empty data map", func(t *testing.T) {
		s := &snapshot.Snapshot{
			ID:            "snap-001",
			Timestamp:     time.Now(),
			SchemaVersion: snapshot.SchemaVersion,
			Source:        snapshot.SourceManual,
			Data:          map[string]any{},
		}
		if !s.IsEmpty() {
			t.Error("IsEmpty() should be true for an empty Data map")
		}
	})

	t.Run("nil data map", func(t *testing.T) {
		s := &snapshot.Snapshot{Data: nil}
		if !s.IsEmpty() {
			t.Error("IsEmpty() should be true when Data is nil")
		}
	})

	t.Run("populated data map", func(t *testing.T) {
		s := &snapshot.Snapshot{
			Data: map[string]any{"os": "linux"},
		}
		if s.IsEmpty() {
			t.Error("IsEmpty() should be false when Data has entries")
		}
	})
}

func TestSnapshot_HasParent(t *testing.T) {
	t.Run("no parent", func(t *testing.T) {
		s := &snapshot.Snapshot{ParentID: ""}
		if s.HasParent() {
			t.Error("HasParent() should be false when ParentID is empty")
		}
	})

	t.Run("with parent", func(t *testing.T) {
		s := &snapshot.Snapshot{ParentID: "snap-000"}
		if !s.HasParent() {
			t.Error("HasParent() should be true when ParentID is set")
		}
	})
}

func TestSnapshot_SourceConstants(t *testing.T) {
	sources := []snapshot.Source{
		snapshot.SourceManual,
		snapshot.SourceRepro,
		snapshot.SourceTimeCapsule,
		snapshot.SourceWatchdog,
	}
	for _, src := range sources {
		if src == "" {
			t.Errorf("Source constant must not be empty string")
		}
	}
}

func TestSchemaVersion_IsPositive(t *testing.T) {
	if snapshot.SchemaVersion <= 0 {
		t.Errorf("SchemaVersion = %d; want > 0", snapshot.SchemaVersion)
	}
}

func TestNewID_FormatAndUniqueness(t *testing.T) {
	id1 := snapshot.NewID()
	id2 := snapshot.NewID()

	if len(id1) != 36 {
		t.Errorf("expected UUID length 36, got %d (%q)", len(id1), id1)
	}
	if id1 == id2 {
		t.Errorf("expected unique IDs, got identical %q", id1)
	}
}
