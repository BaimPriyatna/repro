package event_test

import (
	"strings"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestEvent_HasSnapshot_False(t *testing.T) {
	e := &event.Event{
		ID:        "evt-001",
		Timestamp: time.Now(),
		Type:      event.TypeCommandExecuted,
		Source:    event.SourceUser,
		Subject:   "go build ./...",
	}
	if e.HasSnapshot() {
		t.Error("HasSnapshot() should be false when RelatedSnapshotID is empty")
	}
}

func TestEvent_HasSnapshot_True(t *testing.T) {
	e := &event.Event{
		ID:                "evt-002",
		Timestamp:         time.Now(),
		Type:              event.TypeSnapshotCreated,
		Source:            event.SourceRepro,
		Subject:           "snap-001",
		RelatedSnapshotID: snapshot.ID("snap-001"),
	}
	if !e.HasSnapshot() {
		t.Error("HasSnapshot() should be true when RelatedSnapshotID is set")
	}
}

func TestEventTypeConstants_NonEmpty(t *testing.T) {
	types := []event.Type{
		event.TypeFileChanged,
		event.TypePackageInstalled,
		event.TypePackageRemoved,
		event.TypeConfigChanged,
		event.TypeGitCommit,
		event.TypeCommandExecuted,
		event.TypeRuntimeChanged,
		event.TypeSnapshotCreated,
		event.TypeSnapshotDeleted,
	}
	for _, tp := range types {
		if tp == "" {
			t.Errorf("Event Type constant must not be empty string")
		}
	}
}

func TestEventSourceConstants_NonEmpty(t *testing.T) {
	sources := []event.Source{
		event.SourceWatchdog,
		event.SourceRepro,
		event.SourceTimeCapsule,
		event.SourceUser,
	}
	for _, src := range sources {
		if src == "" {
			t.Errorf("Event Source constant must not be empty string")
		}
	}
}

// TestEvent_NoSnapshotRequired verifies that an Event is valid without a
// RelatedSnapshotID (, — Events are not children of
// Snapshots; they may exist with no snapshot association at all).
func TestEvent_NoSnapshotRequired(t *testing.T) {
	e := &event.Event{
		ID:        "evt-standalone",
		Timestamp: time.Now(),
		Type:      event.TypeCommandExecuted,
		Source:    event.SourceWatchdog,
		Subject:   "npm install",
	}
	if e.RelatedSnapshotID != "" {
		t.Error("RelatedSnapshotID should be empty by default (not mandatory)")
	}
}

func TestNewID(t *testing.T) {
	id1 := event.NewID()
	id2 := event.NewID()

	if id1 == "" || id2 == "" {
		t.Fatal("NewID returned empty string")
	}
	if id1 == id2 {
		t.Fatalf("NewID generated identical IDs: %s", id1)
	}

	parts := strings.Split(string(id1), "-")
	if len(parts) != 5 {
		t.Fatalf("NewID does not match UUID format: %s", id1)
	}
	// Check UUID v4 version bit (char 14 should be '4')
	if parts[2][0] != '4' {
		t.Errorf("UUID is not v4: %s", id1)
	}
}

func TestNew_Success(t *testing.T) {
	fixedTime := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	meta := map[string]any{"path": "foo.go", "lines": 42}

	evt, err := event.New(
		event.TypeFileChanged,
		event.SourceWatchdog,
		"foo.go",
		event.WithID("custom-id-123"),
		event.WithTimestamp(fixedTime),
		event.WithMetadata(meta),
		event.WithSnapshot("snap-999"),
	)
	if err != nil {
		t.Fatalf("unexpected error creating event: %v", err)
	}

	if evt.ID != "custom-id-123" {
		t.Errorf("expected ID custom-id-123, got %s", evt.ID)
	}
	if !evt.Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, evt.Timestamp)
	}
	if evt.Type != event.TypeFileChanged {
		t.Errorf("expected Type %s, got %s", event.TypeFileChanged, evt.Type)
	}
	if evt.Source != event.SourceWatchdog {
		t.Errorf("expected Source %s, got %s", event.SourceWatchdog, evt.Source)
	}
	if evt.Subject != "foo.go" {
		t.Errorf("expected Subject foo.go, got %s", evt.Subject)
	}
	if evt.RelatedSnapshotID != "snap-999" {
		t.Errorf("expected RelatedSnapshotID snap-999, got %s", evt.RelatedSnapshotID)
	}
	if !evt.HasSnapshot() {
		t.Error("expected HasSnapshot() to be true")
	}

	// Verify metadata defensive copy
	meta["lines"] = 999
	if evt.Metadata["lines"] != 42 {
		t.Error("mutating original metadata affected event metadata")
	}
}

func TestNew_Defaults(t *testing.T) {
	evt, err := event.New(event.TypeCommandExecuted, event.SourceUser, "make test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evt.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if evt.Timestamp.IsZero() {
		t.Error("expected auto-generated UTC timestamp")
	}
	if evt.RelatedSnapshotID != "" {
		t.Error("expected empty RelatedSnapshotID by default")
	}
	if evt.HasSnapshot() {
		t.Error("expected HasSnapshot() to be false")
	}
}

func TestValidate_Errors(t *testing.T) {
	var nilEvt *event.Event
	if err := nilEvt.Validate(); err == nil || !errors.Is(err, errors.CodeInvalidInput) {
		t.Errorf("expected CodeInvalidInput for nil event, got %v", err)
	}

	tests := []struct {
		name string
		evt  *event.Event
	}{
		{
			name: "empty ID",
			evt: &event.Event{
				Timestamp: time.Now(),
				Type:      event.TypeGitCommit,
				Source:    event.SourceRepro,
				Subject:   "commit-1",
			},
		},
		{
			name: "zero timestamp",
			evt: &event.Event{
				ID:      "id-1",
				Type:    event.TypeGitCommit,
				Source:  event.SourceRepro,
				Subject: "commit-1",
			},
		},
		{
			name: "empty type",
			evt: &event.Event{
				ID:        "id-1",
				Timestamp: time.Now(),
				Source:    event.SourceRepro,
				Subject:   "commit-1",
			},
		},
		{
			name: "empty source",
			evt: &event.Event{
				ID:        "id-1",
				Timestamp: time.Now(),
				Type:      event.TypeGitCommit,
				Subject:   "commit-1",
			},
		},
		{
			name: "empty subject",
			evt: &event.Event{
				ID:        "id-1",
				Timestamp: time.Now(),
				Type:      event.TypeGitCommit,
				Source:    event.SourceRepro,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.evt.Validate()
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			if !errors.Is(err, errors.CodeInvalidInput) {
				t.Errorf("expected CodeInvalidInput, got %v", err)
			}
		})
	}
}

func TestClone_DeepCopy(t *testing.T) {
	evt, err := event.New(
		event.TypeConfigChanged,
		event.SourceWatchdog,
		"config.toml",
		event.WithMetadata(map[string]any{"key": "value"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cloned := evt.Clone()
	if cloned == nil {
		t.Fatal("expected non-nil clone")
	}

	if cloned.ID != evt.ID || cloned.Subject != evt.Subject {
		t.Error("clone fields mismatch")
	}

	// Mutate clone metadata
	cloned.Metadata["key"] = "modified"
	if evt.Metadata["key"] != "value" {
		t.Error("mutating clone metadata affected original event")
	}

	var nilEvt *event.Event
	if nilEvt.Clone() != nil {
		t.Error("expected nil clone for nil event")
	}
}
