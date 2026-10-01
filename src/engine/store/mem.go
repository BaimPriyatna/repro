// Package store — in-memory store implementation.
package store

import (
	"context"
	"sort"
	"sync"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
)

// MemStore is an in-memory, thread-safe implementation of Store.
type MemStore struct {
	mu        sync.RWMutex
	snapshots map[snapshot.ID]*snapshot.Snapshot
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		snapshots: make(map[snapshot.ID]*snapshot.Snapshot),
	}
}

// Store persists a snapshot in memory.
func (m *MemStore) Store(_ context.Context, snap *snapshot.Snapshot) error {
	if snap == nil || snap.ID == "" {
		return errors.New(errors.CodeInvalidInput, "snapshot or snapshot ID is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[snap.ID]; exists {
		return errors.New(errors.CodeStorageFailure, "snapshot with this ID already exists (immutable)")
	}

	// Deep clone data to guarantee immutability
	cloned := cloneSnapshot(snap)
	m.snapshots[snap.ID] = cloned
	return nil
}

// Load retrieves a snapshot by ID from memory.
func (m *MemStore) Load(_ context.Context, id snapshot.ID) (*snapshot.Snapshot, error) {
	if id == "" {
		return nil, errors.New(errors.CodeInvalidInput, "snapshot ID is empty")
	}

	m.mu.RLock()
	snap, exists := m.snapshots[id]
	m.mu.RUnlock()

	if !exists {
		return nil, errors.New(errors.CodeNotFound, "snapshot not found")
	}

	// Corruption detection
	valid, err := hasher.VerifyContentHash(snap.Data, snap.ContentHash)
	if err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "verifying content hash failed", err)
	}
	if !valid {
		return nil, errors.New(errors.CodeStorageFailure, "silent corruption detected: content hash mismatch")
	}

	return cloneSnapshot(snap), nil
}

// List returns snapshots matching filter, sorted descending by timestamp.
func (m *MemStore) List(_ context.Context, filter Filter) ([]*snapshot.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*snapshot.Snapshot
	for _, snap := range m.snapshots {
		meta := MetadataFromSnapshot(snap)
		if filter.Matches(meta) {
			matched = append(matched, cloneSnapshot(snap))
		}
	}

	sortSnapshotsDesc(matched)
	if filter.Limit > 0 && len(matched) > filter.Limit {
		matched = matched[:filter.Limit]
	}

	return matched, nil
}

// ListMetadata returns metadata matching filter, sorted descending by timestamp.
func (m *MemStore) ListMetadata(_ context.Context, filter Filter) ([]Metadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []Metadata
	for _, snap := range m.snapshots {
		meta := MetadataFromSnapshot(snap)
		if filter.Matches(meta) {
			matched = append(matched, meta)
		}
	}

	sortMetadataDesc(matched)
	if filter.Limit > 0 && len(matched) > filter.Limit {
		matched = matched[:filter.Limit]
	}

	return matched, nil
}

// Delete removes a snapshot by ID.
func (m *MemStore) Delete(_ context.Context, id snapshot.ID) error {
	if id == "" {
		return errors.New(errors.CodeInvalidInput, "snapshot ID is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[id]; !exists {
		return errors.New(errors.CodeNotFound, "snapshot not found")
	}

	delete(m.snapshots, id)
	return nil
}

// Exists checks if a snapshot exists.
func (m *MemStore) Exists(_ context.Context, id snapshot.ID) (bool, error) {
	if id == "" {
		return false, nil
	}

	m.mu.RLock()
	_, exists := m.snapshots[id]
	m.mu.RUnlock()

	return exists, nil
}

func cloneSnapshot(s *snapshot.Snapshot) *snapshot.Snapshot {
	cloned := *s
	if s.Data != nil {
		cloned.Data = make(map[string]any, len(s.Data))
		for k, v := range s.Data {
			cloned.Data[k] = v
		}
	}
	if s.Labels != nil {
		cloned.Labels = make(map[string]string, len(s.Labels))
		for k, v := range s.Labels {
			cloned.Labels[k] = v
		}
	}
	return &cloned
}

func sortSnapshotsDesc(snaps []*snapshot.Snapshot) {
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Timestamp.After(snaps[j].Timestamp)
	})
}

func sortMetadataDesc(metas []Metadata) {
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Timestamp.After(metas[j].Timestamp)
	})
}
