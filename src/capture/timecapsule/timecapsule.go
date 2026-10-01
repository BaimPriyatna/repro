// Package timecapsule maintains periodic and manual historical environment snapshots.
package timecapsule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// TimeCapsule manages periodic and manual historical snapshots.
type TimeCapsule struct {
	cfg *capture.Config

	// Scheduling state
	mu            sync.RWMutex
	running       bool
	ticker        *time.Ticker
	stopCh        chan struct{}
	interval      time.Duration
	lastCaptureID snapshot.ID
}

// Config holds TimeCapsule-specific configuration.
type Config struct {
	// Base capture configuration (required)
	Capture *capture.Config

	// Interval for periodic snapshots (required for Start)
	Interval time.Duration

	// AutoStart determines whether to start periodic capture on New
	AutoStart bool

	// RetainCount is the maximum number of snapshots to retain (0 = unlimited)
	RetainCount int
}

// New creates a new TimeCapsule with the provided configuration.
func New(cfg *Config) (*TimeCapsule, error) {
	if cfg == nil || cfg.Capture == nil {
		return nil, capture.ErrEngineRequired
	}

	if err := cfg.Capture.Validate(); err != nil {
		return nil, err
	}

	tc := &TimeCapsule{
		cfg:      cfg.Capture,
		interval: cfg.Interval,
		stopCh:   make(chan struct{}),
	}

	if cfg.AutoStart && cfg.Interval > 0 {
		if err := tc.Start(cfg.Interval); err != nil {
			return nil, err
		}
	}

	return tc, nil
}

// CaptureOption configures a single TimeCapsule capture operation.
type CaptureOption func(*captureOptions)

type captureOptions struct {
	labels   map[string]string
	parentID snapshot.ID
	reason   string
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

// Capture performs a manual snapshot, stores it, and returns the result.
func (tc *TimeCapsule) Capture(ctx context.Context, opts ...CaptureOption) (*capture.Result, error) {
	options := captureOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	// Merge module-level config labels with call-specific labels
	mergedLabels := make(map[string]string)
	for k, v := range tc.cfg.Labels {
		mergedLabels[k] = v
	}
	for k, v := range options.labels {
		mergedLabels[k] = v
	}

	// Add reason and source labels
	if options.reason != "" {
		mergedLabels["reason"] = options.reason
	}
	mergedLabels["capture_type"] = "manual"

	// Determine parent: use specified parent, or link to last captured snapshot
	parentID := options.parentID
	if parentID == "" {
		tc.mu.RLock()
		parentID = tc.lastCaptureID
		tc.mu.RUnlock()
	}

	snap, err := tc.cfg.Engine.Capture(
		ctx,
		snapshot.SourceTimeCapsule,
		tc.cfg.CaptureOptions(parentID)...,
	)
	if err != nil {
		return &capture.Result{Error: err}, err
	}

	// Store the snapshot
	if err := tc.cfg.Engine.Store(ctx, snap); err != nil {
		return &capture.Result{Snapshot: snap, Error: err}, err
	}

	// Update last capture ID
	tc.mu.Lock()
	tc.lastCaptureID = snap.ID
	tc.mu.Unlock()

	result := &capture.Result{
		Snapshot: snap,
		Events:   make([]*event.Event, 0, 1),
	}

	// Emit SNAPSHOT_CREATED event
	if tc.cfg.EventSystem != nil {
		evt, err := tc.cfg.EventSystem.Emit(
			ctx,
			event.TypeSnapshotCreated,
			event.SourceTimeCapsule,
			fmt.Sprintf("snapshot:%s", snap.ID),
			event.WithSnapshot(snap.ID),
			event.WithMetadata(map[string]any{
				"reason":       options.reason,
				"capture_type": "manual",
			}),
		)
		if err != nil {
			// Event emission failure is non-fatal
			result.Error = err
		} else {
			result.Events = append(result.Events, evt)
		}
	}

	return result, nil
}

// Start begins periodic snapshot capture at the specified interval.
// Returns an error if TimeCapsule is already running.
func (tc *TimeCapsule) Start(interval time.Duration) error {
	if interval <= 0 {
		return capture.ErrInvalidInterval
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.running {
		return capture.ErrAlreadyRunning
	}

	tc.interval = interval
	tc.ticker = time.NewTicker(interval)
	tc.running = true

	go tc.periodicCaptureLoop()

	return nil
}

// Stop halts periodic snapshot capture.
// Returns an error if TimeCapsule is not running.
func (tc *TimeCapsule) Stop() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.running {
		return capture.ErrNotRunning
	}

	tc.running = false
	if tc.ticker != nil {
		tc.ticker.Stop()
		tc.ticker = nil
	}

	close(tc.stopCh)
	tc.stopCh = make(chan struct{}) // reset for potential restart

	return nil
}

// IsRunning returns true if periodic capture is active.
func (tc *TimeCapsule) IsRunning() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.running
}

// LastCaptureID returns the ID of the most recent snapshot captured by this TimeCapsule.
func (tc *TimeCapsule) LastCaptureID() snapshot.ID {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.lastCaptureID
}

// periodicCaptureLoop runs in a goroutine and performs scheduled captures.
func (tc *TimeCapsule) periodicCaptureLoop() {
	tc.mu.RLock()
	ticker := tc.ticker
	stopCh := tc.stopCh
	tc.mu.RUnlock()

	if ticker == nil {
		return
	}

	for {
		select {
		case <-ticker.C:
			// Perform scheduled capture; errors are ignored to continue running.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			_, _ = tc.Capture(ctx, WithReason("scheduled"))
			cancel()

		case <-stopCh:
			return
		}
	}
}

// Close stops the TimeCapsule and releases resources.
func (tc *TimeCapsule) Close() error {
	tc.mu.RLock()
	running := tc.running
	tc.mu.RUnlock()

	if running {
		return tc.Stop()
	}
	return nil
}
