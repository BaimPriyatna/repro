// Package store — filesystem-backed snapshot store implementation.
package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
)

// FileStore is a persistent filesystem-backed implementation of Store.
// Snapshots are stored as JSON files under <baseDir>/snapshots/<id>.json.
type FileStore struct {
	mu       sync.RWMutex
	baseDir  string
	snapsDir string
}

// NewFileStore creates a FileStore rooted at baseDir.
// It creates the snapshots directory if it does not yet exist.
func NewFileStore(baseDir string) (*FileStore, error) {
	if baseDir == "" {
		return nil, errors.New(errors.CodeInvalidInput, "base directory cannot be empty")
	}

	snapsDir := filepath.Join(baseDir, "snapshots")
	if err := os.MkdirAll(snapsDir, 0o750); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "creating snapshots directory", err)
	}

	return &FileStore{
		baseDir:  baseDir,
		snapsDir: snapsDir,
	}, nil
}

// snapshotPath returns the file path for a snapshot ID.
func (f *FileStore) snapshotPath(id snapshot.ID) string {
	return filepath.Join(f.snapsDir, string(id)+".json")
}

// Store persists a snapshot to disk atomically.
func (f *FileStore) Store(_ context.Context, snap *snapshot.Snapshot) error {
	if snap == nil || snap.ID == "" {
		return errors.New(errors.CodeInvalidInput, "snapshot or snapshot ID is empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	targetPath := f.snapshotPath(snap.ID)
	if _, err := os.Stat(targetPath); err == nil {
		return errors.New(errors.CodeStorageFailure, "snapshot with this ID already exists on disk (immutable)")
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "marshaling snapshot JSON", err)
	}

	// Atomic write: write to temp file first, then rename
	tmpFile, err := os.CreateTemp(f.snapsDir, "tmp_snap_*.json")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "creating temporary snapshot file", err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "writing temporary snapshot file", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "closing temporary snapshot file", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "renaming temporary snapshot file", err)
	}

	return nil
}

// Load retrieves and validates a snapshot from disk.
func (f *FileStore) Load(_ context.Context, id snapshot.ID) (*snapshot.Snapshot, error) {
	if id == "" {
		return nil, errors.New(errors.CodeInvalidInput, "snapshot ID is empty")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	targetPath := f.snapshotPath(id)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeNotFound, "snapshot not found on disk")
		}
		return nil, errors.Wrap(errors.CodeStorageFailure, "reading snapshot file", err)
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "unmarshaling snapshot JSON", err)
	}

	// Corruption detection
	valid, err := hasher.VerifyContentHash(snap.Data, snap.ContentHash)
	if err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "verifying content hash failed", err)
	}
	if !valid {
		return nil, errors.New(errors.CodeStorageFailure, "silent corruption detected: content hash mismatch")
	}

	return &snap, nil
}

// List returns snapshots matching filter, sorted descending by timestamp.
func (f *FileStore) List(ctx context.Context, filter Filter) ([]*snapshot.Snapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.snapsDir)
	if err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "reading snapshots directory", err)
	}

	var snaps []*snapshot.Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := snapshot.ID(strings.TrimSuffix(entry.Name(), ".json"))
		snap, err := f.loadUnlocked(id)
		if err != nil {
			continue // skip corrupted or unreadable entries during listing
		}

		meta := MetadataFromSnapshot(snap)
		if filter.Matches(meta) {
			snaps = append(snaps, snap)
		}
	}

	sortSnapshotsDesc(snaps)
	if filter.Limit > 0 && len(snaps) > filter.Limit {
		snaps = snaps[:filter.Limit]
	}

	return snaps, nil
}

// ListMetadata returns lightweight metadata summaries matching filter, sorted descending by timestamp.
func (f *FileStore) ListMetadata(_ context.Context, filter Filter) ([]Metadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.snapsDir)
	if err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "reading snapshots directory", err)
	}

	var metas []Metadata
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := snapshot.ID(strings.TrimSuffix(entry.Name(), ".json"))
		snap, err := f.loadUnlocked(id)
		if err != nil {
			continue
		}

		meta := MetadataFromSnapshot(snap)
		if filter.Matches(meta) {
			metas = append(metas, meta)
		}
	}

	sortMetadataDesc(metas)
	if filter.Limit > 0 && len(metas) > filter.Limit {
		metas = metas[:filter.Limit]
	}

	return metas, nil
}

// Delete removes a snapshot file from disk.
func (f *FileStore) Delete(_ context.Context, id snapshot.ID) error {
	if id == "" {
		return errors.New(errors.CodeInvalidInput, "snapshot ID is empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	targetPath := f.snapshotPath(id)
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.CodeNotFound, "snapshot file not found on disk")
		}
		return errors.Wrap(errors.CodeStorageFailure, "checking snapshot existence", err)
	}

	if err := os.Remove(targetPath); err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "removing snapshot file", err)
	}

	return nil
}

// Exists checks if a snapshot file exists on disk.
func (f *FileStore) Exists(_ context.Context, id snapshot.ID) (bool, error) {
	if id == "" {
		return false, nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	targetPath := f.snapshotPath(id)
	_, err := os.Stat(targetPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, errors.Wrap(errors.CodeStorageFailure, "checking snapshot file existence", err)
}

func (f *FileStore) loadUnlocked(id snapshot.ID) (*snapshot.Snapshot, error) {
	targetPath := f.snapshotPath(id)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}
