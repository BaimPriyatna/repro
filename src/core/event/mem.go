// Package event — in-memory event store implementation.
package event

import (
	"context"
	"sync"

	"github.com/BaimPriyatna/repro/src/core/errors"
)

// MemStore is a thread-safe in-memory implementation of Store.
// Useful for unit tests, ephemeral captures, and isolated environments.
type MemStore struct {
	mu     sync.RWMutex
	events map[ID]*Event
}

// NewMemStore creates an empty, ready-to-use MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		events: make(map[ID]*Event),
	}
}

// Record persists an event in memory.
// Returns CodeStorageFailure if an event with the same ID already exists.
func (m *MemStore) Record(_ context.Context, evt *Event) error {
	if evt == nil {
		return errors.New(errors.CodeInvalidInput, "cannot record nil event")
	}
	if err := evt.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.events[evt.ID]; exists {
		return errors.New(errors.CodeStorageFailure, "event with this ID already exists (immutable)")
	}

	m.events[evt.ID] = evt.Clone()
	return nil
}

// Get retrieves a deep copy of an event by ID.
func (m *MemStore) Get(_ context.Context, id ID) (*Event, error) {
	if id == "" {
		return nil, errors.New(errors.CodeInvalidInput, "event ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	evt, exists := m.events[id]
	if !exists {
		return nil, errors.New(errors.CodeNotFound, "event not found")
	}

	return evt.Clone(), nil
}

// Query returns events matching query criteria, ordered by timestamp.
func (m *MemStore) Query(_ context.Context, q Query) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*Event
	for _, evt := range m.events {
		if q.Matches(evt) {
			matches = append(matches, evt.Clone())
		}
	}

	SortByTimestamp(matches, q.Order)

	if q.Offset > 0 {
		if q.Offset >= len(matches) {
			return []*Event{}, nil
		}
		matches = matches[q.Offset:]
	}

	if q.Limit > 0 && len(matches) > q.Limit {
		matches = matches[:q.Limit]
	}

	if matches == nil {
		matches = []*Event{}
	}

	return matches, nil
}

// Count returns the number of events matching query criteria.
func (m *MemStore) Count(_ context.Context, q Query) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, evt := range m.events {
		if q.Matches(evt) {
			count++
		}
	}
	return count, nil
}

// Delete removes an event by ID.
func (m *MemStore) Delete(_ context.Context, id ID) error {
	if id == "" {
		return errors.New(errors.CodeInvalidInput, "event ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.events[id]; !exists {
		return errors.New(errors.CodeNotFound, "event not found")
	}

	delete(m.events, id)
	return nil
}

// Exists reports whether an event with the given ID exists.
func (m *MemStore) Exists(_ context.Context, id ID) (bool, error) {
	if id == "" {
		return false, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.events[id]
	return exists, nil
}

// Close implements Store. Close on MemStore is a no-op.
func (m *MemStore) Close() error {
	return nil
}
