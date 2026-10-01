// Package event — storage interface for events.
package event

import (
	"context"
)

// Store defines the storage and querying contract for Repro events.
// Implementations must be thread-safe.
type Store interface {
	// Record persists an event.
	// Returns CodeInvalidInput if event validation fails.
	Record(ctx context.Context, evt *Event) error

	// Get retrieves an event by its unique ID.
	// Returns CodeNotFound if the event does not exist.
	// Returns CodeStorageFailure if corruption is detected.
	Get(ctx context.Context, id ID) (*Event, error)

	// Query returns all events matching the query criteria, ordered according to q.Order.
	Query(ctx context.Context, q Query) ([]*Event, error)

	// Count returns the total number of events matching the query criteria.
	Count(ctx context.Context, q Query) (int, error)

	// Delete removes an event by ID.
	// Returns CodeNotFound if the event does not exist.
	Delete(ctx context.Context, id ID) error

	// Exists reports whether an event with the given ID exists.
	Exists(ctx context.Context, id ID) (bool, error)

	// Close releases any resources held by the store (e.g. open file handles or background flushers).
	Close() error
}
