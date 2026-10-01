// Package snapshot defines the core Snapshot domain type for Repro.
//
// A Snapshot is the primary representation of development environment state at a
// specific point in time. Snapshots are immutable once created and are the
// source of truth for all analysis modules.
package snapshot

import (
	"crypto/rand"
	"fmt"
	"time"
)

// SchemaVersion is the current version of the Snapshot schema.
// Increment this when the schema changes in a backward-incompatible way.
const SchemaVersion = 1

// Source describes why a snapshot was created.
type Source string

const (
	// SourceManual indicates a snapshot created by an explicit user request.
	SourceManual Source = "manual"

	// SourceRepro indicates a snapshot created by the Repro capture module.
	SourceRepro Source = "repro"

	// SourceTimeCapsule indicates a snapshot created by the TimeCapsule module.
	SourceTimeCapsule Source = "timecapsule"

	// SourceWatchdog indicates a snapshot triggered by the Watchdog sensor.
	SourceWatchdog Source = "watchdog"
)

// ID is a stable, unique snapshot identifier.
// Implementations must guarantee global uniqueness across the local store.
type ID string

// NewID generates a cryptographically secure random UUID v4 identifier.
func NewID() ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return ID(fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}

// Snapshot is a structured representation of relevant system, environment, and
// project state at a specific point in time.
//
// Snapshots must be treated as immutable once stored.
// No field may be silently changed after creation.
type Snapshot struct {
	// ID is the unique identifier for this snapshot.
	ID ID `json:"id"`

	// Timestamp is the wall-clock time at which the snapshot was created.
	Timestamp time.Time `json:"timestamp"`

	// SchemaVersion is the version of the Snapshot schema used when this
	// snapshot was captured. Used for forward-compatibility.
	SchemaVersion int `json:"schema_version"`

	// ParentID optionally references the immediately preceding snapshot in the
	// same session or timeline. An empty string means "no parent".
	// This is a reference only — no hard foreign-key semantics are assumed.
	ParentID ID `json:"parent_id,omitempty"`

	// ContentHash is a deterministic hash of the snapshot Data payload
	// . Used to detect silent corruption and to deduplicate
	// identical snapshots.
	ContentHash string `json:"content_hash"`

	// Source records why this snapshot was created.
	Source Source `json:"source"`

	// Data holds the captured state, keyed by collector name.
	// The concrete value type for each key is determined by the collector that
	// produced it. The storage layer must preserve unknown keys without
	// modification.
	Data map[string]any `json:"data"`

	// Labels is an optional set of user-defined or system-defined metadata
	// tags. Labels must not affect snapshot identity or content hash.
	Labels map[string]string `json:"labels,omitempty"`
}

// IsEmpty reports whether the snapshot carries no collected data.
func (s *Snapshot) IsEmpty() bool {
	return len(s.Data) == 0
}

// HasParent reports whether this snapshot declares a parent relationship.
func (s *Snapshot) HasParent() bool {
	return s.ParentID != ""
}
