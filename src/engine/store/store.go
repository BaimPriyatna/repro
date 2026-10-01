// Package store defines the storage interface and implementations for snapshots.
package store

import (
	"context"
	"time"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Metadata provides a lightweight summary of a snapshot without its full data payload.
type Metadata struct {
	ID            snapshot.ID       `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	SchemaVersion int               `json:"schema_version"`
	ParentID      snapshot.ID       `json:"parent_id,omitempty"`
	ContentHash   string            `json:"content_hash"`
	Source        snapshot.Source   `json:"source"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// MetadataFromSnapshot extracts lightweight metadata from a snapshot.
func MetadataFromSnapshot(s *snapshot.Snapshot) Metadata {
	labels := make(map[string]string, len(s.Labels))
	for k, v := range s.Labels {
		labels[k] = v
	}
	return Metadata{
		ID:            s.ID,
		Timestamp:     s.Timestamp,
		SchemaVersion: s.SchemaVersion,
		ParentID:      s.ParentID,
		ContentHash:   s.ContentHash,
		Source:        s.Source,
		Labels:        labels,
	}
}

// Filter specifies criteria when querying snapshots.
type Filter struct {
	// Source filters snapshots created by a specific source.
	Source snapshot.Source

	// ParentID filters snapshots having a specific parent snapshot ID.
	ParentID snapshot.ID

	// CreatedBefore filters snapshots created strictly before this time.
	CreatedBefore time.Time

	// CreatedAfter filters snapshots created strictly after this time.
	CreatedAfter time.Time

	// Limit caps the number of results returned (0 means unlimited).
	Limit int
}

// Matches evaluates whether the given metadata satisfies the filter criteria.
func (f Filter) Matches(meta Metadata) bool {
	if f.Source != "" && meta.Source != f.Source {
		return false
	}
	if f.ParentID != "" && meta.ParentID != f.ParentID {
		return false
	}
	if !f.CreatedBefore.IsZero() && !meta.Timestamp.Before(f.CreatedBefore) {
		return false
	}
	if !f.CreatedAfter.IsZero() && !meta.Timestamp.After(f.CreatedAfter) {
		return false
	}
	return true
}

// Store defines the snapshot persistence contract.
// Implementations must be thread-safe.
type Store interface {
	// Store persists a snapshot.
	Store(ctx context.Context, snap *snapshot.Snapshot) error

	// Load retrieves a snapshot by ID.
	// Returns CodeNotFound if the snapshot does not exist.
	// Verifies content hash and returns CodeStorageFailure if silent corruption is detected.
	Load(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error)

	// List returns all snapshots satisfying the filter, ordered by timestamp descending (newest first).
	List(ctx context.Context, filter Filter) ([]*snapshot.Snapshot, error)

	// ListMetadata returns metadata summaries matching the filter, ordered by timestamp descending.
	ListMetadata(ctx context.Context, filter Filter) ([]Metadata, error)

	// Delete removes a snapshot by ID.
	// Returns CodeNotFound if the snapshot does not exist.
	Delete(ctx context.Context, id snapshot.ID) error

	// Exists reports whether a snapshot with the given ID exists.
	Exists(ctx context.Context, id snapshot.ID) (bool, error)
}
