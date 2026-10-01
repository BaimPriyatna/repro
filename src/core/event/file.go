// Package event — filesystem-backed event store implementation.
package event

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// eventMeta is a lightweight in-memory index entry for fast querying.
type eventMeta struct {
	ID                ID
	Timestamp         time.Time
	Type              Type
	Source            Source
	Subject           string
	RelatedSnapshotID snapshot.ID
}

// FileStore is a persistent filesystem-backed implementation of Store.
// Events are stored as JSON files under <baseDir>/events/<id>.json.
// An in-memory metadata index is maintained for efficient querying and ordering.
type FileStore struct {
	mu        sync.RWMutex
	baseDir   string
	eventsDir string
	index     map[ID]eventMeta
}

// NewFileStore creates a FileStore rooted at baseDir.
// It creates the events directory if it does not yet exist, and scans existing
// events to build the in-memory metadata index for efficient querying.
func NewFileStore(baseDir string) (*FileStore, error) {
	if baseDir == "" {
		return nil, errors.New(errors.CodeInvalidInput, "base directory cannot be empty")
	}

	eventsDir := filepath.Join(baseDir, "events")
	if err := os.MkdirAll(eventsDir, 0o750); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "creating events directory", err)
	}

	fs := &FileStore{
		baseDir:   baseDir,
		eventsDir: eventsDir,
		index:     make(map[ID]eventMeta),
	}

	if err := fs.rebuildIndex(); err != nil {
		return nil, err
	}

	return fs, nil
}

// sanitizeID verifies that an event ID does not contain path traversal characters.
func sanitizeID(id ID) error {
	str := string(id)
	if str == "" {
		return errors.New(errors.CodeInvalidInput, "event ID cannot be empty")
	}
	if strings.ContainsAny(str, `/\:*?"<>|`) || filepath.Base(str) != str || str == "." || str == ".." {
		return errors.New(errors.CodeInvalidInput, "invalid characters or path traversal in event ID")
	}
	return nil
}

// eventPath returns the file path for an event ID.
func (f *FileStore) eventPath(id ID) string {
	return filepath.Join(f.eventsDir, string(id)+".json")
}

// rebuildIndex scans the events directory and populates the in-memory index.
func (f *FileStore) rebuildIndex() error {
	entries, err := os.ReadDir(f.eventsDir)
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "reading events directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), "tmp_") {
			continue
		}

		filePath := filepath.Join(f.eventsDir, entry.Name())
		data, err := os.ReadFile(filePath) //nolint:gosec // filePath is built from internal eventsDir
		if err != nil {
			continue // skip unreadable files during initial index build
		}

		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			continue // skip malformed files during indexing; detected on explicit Get
		}

		if evt.ID != "" && !evt.Timestamp.IsZero() {
			f.index[evt.ID] = eventMeta{
				ID:                evt.ID,
				Timestamp:         evt.Timestamp,
				Type:              evt.Type,
				Source:            evt.Source,
				Subject:           evt.Subject,
				RelatedSnapshotID: evt.RelatedSnapshotID,
			}
		}
	}

	return nil
}

// Record persists an event to disk atomically and updates the index.
func (f *FileStore) Record(_ context.Context, evt *Event) error {
	if evt == nil {
		return errors.New(errors.CodeInvalidInput, "cannot record nil event")
	}
	if err := evt.Validate(); err != nil {
		return err
	}
	if err := sanitizeID(evt.ID); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	targetPath := f.eventPath(evt.ID)
	if _, exists := f.index[evt.ID]; exists {
		return errors.New(errors.CodeStorageFailure, "event with this ID already exists on disk (immutable)")
	}
	if _, err := os.Stat(targetPath); err == nil {
		return errors.New(errors.CodeStorageFailure, "event file already exists on disk (immutable)")
	}

	data, err := json.MarshalIndent(evt, "", "  ")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "marshaling event JSON", err)
	}

	// Atomic write: write to temp file first, then rename
	tmpFile, err := os.CreateTemp(f.eventsDir, "tmp_evt_*.json")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "creating temporary event file", err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "writing temporary event file", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "closing temporary event file", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "renaming temporary event file", err)
	}

	f.index[evt.ID] = eventMeta{
		ID:                evt.ID,
		Timestamp:         evt.Timestamp,
		Type:              evt.Type,
		Source:            evt.Source,
		Subject:           evt.Subject,
		RelatedSnapshotID: evt.RelatedSnapshotID,
	}

	return nil
}

// Get retrieves an event from disk and verifies its integrity.
func (f *FileStore) Get(_ context.Context, id ID) (*Event, error) {
	if err := sanitizeID(id); err != nil {
		return nil, err
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.loadUnlocked(id)
}

// loadUnlocked reads and unmarshals an event without acquiring locks.
func (f *FileStore) loadUnlocked(id ID) (*Event, error) {
	targetPath := f.eventPath(id)
	data, err := os.ReadFile(targetPath) //nolint:gosec // targetPath is constructed safely via sanitizeID
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeNotFound, "event not found on disk")
		}
		return nil, errors.Wrap(errors.CodeStorageFailure, "reading event file", err)
	}

	var evt Event
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "silent corruption detected: unmarshaling event JSON", err)
	}

	if err := evt.Validate(); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "silent corruption detected: invalid event data on disk", err)
	}

	return &evt, nil
}

// Query returns events matching query criteria, ordered by timestamp.
func (f *FileStore) Query(_ context.Context, q Query) ([]*Event, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// 1. Identify matching metadata entries
	var matchingMetas []eventMeta
	for _, meta := range f.index {
		fakeEvt := Event{
			ID:                meta.ID,
			Timestamp:         meta.Timestamp,
			Type:              meta.Type,
			Source:            meta.Source,
			Subject:           meta.Subject,
			RelatedSnapshotID: meta.RelatedSnapshotID,
		}
		if q.Matches(&fakeEvt) {
			matchingMetas = append(matchingMetas, meta)
		}
	}

	// 2. Sort according to query order
	if q.Order == SortAsc {
		sortMetasAsc(matchingMetas)
	} else {
		sortMetasDesc(matchingMetas)
	}

	// 3. Apply Offset and Limit
	if q.Offset > 0 {
		if q.Offset >= len(matchingMetas) {
			return []*Event{}, nil
		}
		matchingMetas = matchingMetas[q.Offset:]
	}

	if q.Limit > 0 && len(matchingMetas) > q.Limit {
		matchingMetas = matchingMetas[:q.Limit]
	}

	// 4. Load full events for the matching results
	results := make([]*Event, 0, len(matchingMetas))
	for _, meta := range matchingMetas {
		evt, err := f.loadUnlocked(meta.ID)
		if err != nil {
			continue // skip corrupted file during query traversal
		}
		results = append(results, evt)
	}

	return results, nil
}

// Count returns the number of events matching query criteria.
func (f *FileStore) Count(_ context.Context, q Query) (int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	count := 0
	for _, meta := range f.index {
		fakeEvt := Event{
			ID:                meta.ID,
			Timestamp:         meta.Timestamp,
			Type:              meta.Type,
			Source:            meta.Source,
			Subject:           meta.Subject,
			RelatedSnapshotID: meta.RelatedSnapshotID,
		}
		if q.Matches(&fakeEvt) {
			count++
		}
	}
	return count, nil
}

// Delete removes an event by ID from both disk and index.
func (f *FileStore) Delete(_ context.Context, id ID) error {
	if err := sanitizeID(id); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	targetPath := f.eventPath(id)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return errors.New(errors.CodeNotFound, "event not found on disk")
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(errors.CodeStorageFailure, "deleting event file", err)
	}

	delete(f.index, id)
	return nil
}

// Exists reports whether an event with the given ID exists.
func (f *FileStore) Exists(_ context.Context, id ID) (bool, error) {
	if err := sanitizeID(id); err != nil {
		return false, nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.index[id]
	return exists, nil
}

// Close implements Store. Close on FileStore is a clean no-op.
func (f *FileStore) Close() error {
	return nil
}

func sortMetasAsc(metas []eventMeta) {
	for i := 1; i < len(metas); i++ {
		for j := i; j > 0; j-- {
			if metas[j].Timestamp.Before(metas[j-1].Timestamp) ||
				(metas[j].Timestamp.Equal(metas[j-1].Timestamp) && metas[j].ID < metas[j-1].ID) {
				metas[j], metas[j-1] = metas[j-1], metas[j]
			} else {
				break
			}
		}
	}
}

func sortMetasDesc(metas []eventMeta) {
	for i := 1; i < len(metas); i++ {
		for j := i; j > 0; j-- {
			if metas[j].Timestamp.After(metas[j-1].Timestamp) ||
				(metas[j].Timestamp.Equal(metas[j-1].Timestamp) && metas[j].ID > metas[j-1].ID) {
				metas[j], metas[j-1] = metas[j-1], metas[j]
			} else {
				break
			}
		}
	}
}
