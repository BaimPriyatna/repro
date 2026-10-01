// Package capture provides shared types and configuration for environment capture modules.
package capture

import (
	"context"

	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
)

// Engine abstracts snapshot capture and storage for capture modules.
type Engine interface {
	// Capture creates a new snapshot using the provided source and options.
	Capture(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error)

	// Store persists a snapshot to the underlying store.
	Store(ctx context.Context, snap *snapshot.Snapshot) error

	// Load retrieves a snapshot by ID.
	Load(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error)
}

// EventSystem abstracts the Event System interface for capture modules.
// Watchdog and other sensors emit events through this interface.
type EventSystem interface {
	// Emit creates and records an event in a single operation.
	Emit(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error)

	// Record persists an existing event.
	Record(ctx context.Context, evt *event.Event) error
}

// Result represents the outcome of a capture operation.
type Result struct {
	// Snapshot is the newly created snapshot (may be nil if capture failed).
	Snapshot *snapshot.Snapshot

	// Events are any events emitted during the capture operation.
	Events []*event.Event

	// Error contains any error that occurred during capture.
	Error error
}

// Config holds common configuration for capture modules.
type Config struct {
	// Engine is the Snapshot Engine (required).
	Engine Engine

	// EventSystem is the Event System (required for Watchdog, optional for others).
	EventSystem EventSystem

	// Labels are default metadata labels to attach to all snapshots.
	Labels map[string]string

	// CollectorNames restricts capture to specific collectors (nil = all collectors).
	CollectorNames []string
}

// Validate checks that required configuration fields are present.
func (c *Config) Validate() error {
	if c.Engine == nil {
		return ErrEngineRequired
	}
	return nil
}

// CaptureOptions builds engine.CaptureOption values from Config.
func (c *Config) CaptureOptions(parentID snapshot.ID) []engine.CaptureOption {
	var opts []engine.CaptureOption

	if parentID != "" {
		opts = append(opts, engine.WithParent(parentID))
	}

	if len(c.Labels) > 0 {
		opts = append(opts, engine.WithLabels(c.Labels))
	}

	if len(c.CollectorNames) > 0 {
		opts = append(opts, engine.WithCollectors(c.CollectorNames...))
	}

	return opts
}
