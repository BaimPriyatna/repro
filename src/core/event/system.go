// Package event — high-level Event System service.
package event

import (
	"context"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// System coordinates event creation, persistence, and querying.
type System struct {
	store Store
}

// NewSystem constructs a new Event System backed by the provided Store.
func NewSystem(store Store) (*System, error) {
	if store == nil {
		return nil, errors.New(errors.CodeInvalidInput, "store cannot be nil")
	}
	return &System{store: store}, nil
}

// Store returns the underlying storage implementation.
func (s *System) Store() Store {
	return s.store
}

// Emit creates, validates, records, and returns an Event in a single operation.
func (s *System) Emit(ctx context.Context, eventType Type, source Source, subject string, opts ...Option) (*Event, error) {
	evt, err := New(eventType, source, subject, opts...)
	if err != nil {
		return nil, err
	}

	if err := s.store.Record(ctx, evt); err != nil {
		return nil, err
	}

	return evt, nil
}

// Record persists an existing event into the store after validation.
func (s *System) Record(ctx context.Context, evt *Event) error {
	return s.store.Record(ctx, evt)
}

// Get retrieves an event by its ID.
func (s *System) Get(ctx context.Context, id ID) (*Event, error) {
	return s.store.Get(ctx, id)
}

// Query returns all events matching query criteria, sorted according to q.Order.
func (s *System) Query(ctx context.Context, q Query) ([]*Event, error) {
	return s.store.Query(ctx, q)
}

// Count returns the number of events matching the query criteria.
func (s *System) Count(ctx context.Context, q Query) (int, error) {
	return s.store.Count(ctx, q)
}

// Delete removes an event by ID.
func (s *System) Delete(ctx context.Context, id ID) error {
	return s.store.Delete(ctx, id)
}

// EventsForSnapshot returns all events associated with a specific snapshot ID,
// ordered chronologically (oldest to newest) to reconstruct the timeline around the snapshot.
func (s *System) EventsForSnapshot(ctx context.Context, snapID snapshot.ID) ([]*Event, error) {
	if snapID == "" {
		return nil, errors.New(errors.CodeInvalidInput, "snapshot ID cannot be empty")
	}

	return s.store.Query(ctx, Query{
		RelatedSnapshotID: snapID,
		Order:             SortAsc,
	})
}

// EventsInTimeWindow returns events occurring between after and before (inclusive).
func (s *System) EventsInTimeWindow(ctx context.Context, after, before time.Time, order SortOrder) ([]*Event, error) {
	return s.store.Query(ctx, Query{
		After:  after,
		Before: before,
		Order:  order,
	})
}

// EventsByType returns events matching the specified type, newest first.
func (s *System) EventsByType(ctx context.Context, eventType Type) ([]*Event, error) {
	return s.store.Query(ctx, Query{
		Type:  eventType,
		Order: SortDesc,
	})
}

// Latest returns the single most recent event matching query criteria, or nil if none match.
func (s *System) Latest(ctx context.Context, q Query) (*Event, error) {
	q.Limit = 1
	q.Order = SortDesc
	events, err := s.store.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	return events[0], nil
}

// Close releases any underlying store resources.
func (s *System) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}
