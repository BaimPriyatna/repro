// Package repro performs one-shot environment capture for reproduction.
package repro

import (
	"context"
	"fmt"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Repro performs one-shot environment captures for reproduction.
type Repro struct {
	cfg *capture.Config
}

// New creates a new Repro capture module with the provided configuration.
func New(cfg *capture.Config) (*Repro, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Repro{cfg: cfg}, nil
}

// CaptureOption configures a single Repro capture operation.
type CaptureOption func(*captureOptions)

type captureOptions struct {
	labels         map[string]string
	parentID       snapshot.ID
	reason         string
	emitEvent      bool
	collectorNames []string
}

// WithLabels attaches metadata labels to the snapshot.
func WithLabels(labels map[string]string) CaptureOption {
	return func(opts *captureOptions) {
		if opts.labels == nil {
			opts.labels = make(map[string]string, len(labels))
		}
		for k, v := range labels {
			opts.labels[k] = v
		}
	}
}

// WithParent sets the parent snapshot reference for historical linkage.
func WithParent(parentID snapshot.ID) CaptureOption {
	return func(opts *captureOptions) {
		opts.parentID = parentID
	}
}

// WithReason documents why this capture was triggered.
func WithReason(reason string) CaptureOption {
	return func(opts *captureOptions) {
		opts.reason = reason
	}
}

// WithEvent causes Repro to emit a SNAPSHOT_CREATED event after successful capture.
func WithEvent() CaptureOption {
	return func(opts *captureOptions) {
		opts.emitEvent = true
	}
}

// WithCollectors restricts the capture to only the specified collector names.
func WithCollectors(names ...string) CaptureOption {
	return func(opts *captureOptions) {
		opts.collectorNames = names
	}
}

// Capture performs a one-shot environment capture and returns the resulting snapshot.
// The snapshot is not stored; callers must call Store explicitly.
func (r *Repro) Capture(ctx context.Context, opts ...CaptureOption) (*capture.Result, error) {
	options := captureOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	// Merge module-level config labels with call-specific labels
	mergedLabels := make(map[string]string)
	for k, v := range r.cfg.Labels {
		mergedLabels[k] = v
	}
	for k, v := range options.labels {
		mergedLabels[k] = v
	}

	// Add reason label if provided
	if options.reason != "" {
		mergedLabels["reason"] = options.reason
	}

	snap, err := r.cfg.Engine.Capture(
		ctx,
		snapshot.SourceRepro,
		r.cfg.CaptureOptions(options.parentID)...,
	)
	if err != nil {
		return &capture.Result{Error: err}, err
	}

	result := &capture.Result{
		Snapshot: snap,
		Events:   make([]*event.Event, 0, 1),
	}

	// Optionally emit SNAPSHOT_CREATED event
	if options.emitEvent && r.cfg.EventSystem != nil {
		evt, err := r.cfg.EventSystem.Emit(
			ctx,
			event.TypeSnapshotCreated,
			event.SourceRepro,
			fmt.Sprintf("snapshot:%s", snap.ID),
			event.WithSnapshot(snap.ID),
			event.WithMetadata(map[string]any{
				"reason": options.reason,
			}),
		)
		if err != nil {
			// Event emission failure is non-fatal; snapshot is still valid
			result.Error = err
		} else {
			result.Events = append(result.Events, evt)
		}
	}

	return result, nil
}

// Store persists a snapshot to the underlying store.
func (r *Repro) Store(ctx context.Context, snap *snapshot.Snapshot) error {
	return r.cfg.Engine.Store(ctx, snap)
}

// CaptureAndStore is a convenience method that captures and immediately persists
// the snapshot in a single operation.
func (r *Repro) CaptureAndStore(ctx context.Context, opts ...CaptureOption) (*capture.Result, error) {
	result, err := r.Capture(ctx, opts...)
	if err != nil {
		return result, err
	}

	if result.Snapshot != nil {
		if err := r.Store(ctx, result.Snapshot); err != nil {
			result.Error = err
			return result, err
		}
	}

	return result, nil
}
