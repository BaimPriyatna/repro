// Package collector defines the Collector interface and built-in environment
// collectors for the Repro Snapshot Engine.
//
// A Collector is responsible for one narrow slice of environment state.
// The engine drives all collectors and merges their output into the
// snapshot Data map, keyed by the collector's Name.
package collector

import (
	"context"
)

// Result is the output of a single collector run.
// The Data field holds the collected state; it must be JSON-serialisable.
// If the collector encountered a non-fatal condition it can set Warning.
type Result struct {
	// Data is the collected state for this collector. The type must be
	// JSON-serialisable (maps, structs with exported fields, scalars).
	Data any

	// Warning is an optional human-readable description of a non-fatal
	// condition (e.g. a tool was unavailable). An empty string means no
	// warning.
	Warning string
}

// Collector is the interface every environment collector must satisfy.
//
// Implementations must be:
//   - deterministic: the same environment must produce the same Data
//   - safe: must not modify the environment
//   - scoped: must collect only what their name implies
type Collector interface {
	// Name returns a stable, unique key used to store this collector's
	// output in Snapshot.Data. Must be a non-empty lowercase identifier
	// with no spaces (e.g. "os", "git", "runtime").
	Name() string

	// Collect gathers the relevant slice of environment state and returns
	// it. A non-nil error means collection failed and should be reported
	// to the caller; a non-empty Warning on a nil-error result means
	// collection partially succeeded.
	//
	// ctx is provided for timeout/cancellation support.
	Collect(ctx context.Context) (Result, error)
}
