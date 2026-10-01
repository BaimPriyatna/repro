// Package engine captures, stores, loads, and compares snapshots.
package engine

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/collector"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
	"github.com/BaimPriyatna/repro/src/engine/normalizer"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

// CaptureConfig holds parameters for a single capture run.
type CaptureConfig struct {
	ParentID snapshot.ID
	Labels   map[string]string
	Subset   map[string]struct{}
}

// CaptureOption configures a snapshot capture operation.
type CaptureOption func(*CaptureConfig)

// WithParent sets the parent snapshot reference.
func WithParent(parentID snapshot.ID) CaptureOption {
	return func(cfg *CaptureConfig) {
		cfg.ParentID = parentID
	}
}

// WithLabels attaches metadata labels to the captured snapshot.
func WithLabels(labels map[string]string) CaptureOption {
	return func(cfg *CaptureConfig) {
		if cfg.Labels == nil {
			cfg.Labels = make(map[string]string, len(labels))
		}
		for k, v := range labels {
			cfg.Labels[k] = v
		}
	}
}

// WithCollectors restricts capture to only the specified collector names.
func WithCollectors(names ...string) CaptureOption {
	return func(cfg *CaptureConfig) {
		cfg.Subset = make(map[string]struct{}, len(names))
		for _, name := range names {
			cfg.Subset[name] = struct{}{}
		}
	}
}

// Option configures the SnapshotEngine upon creation.
type Option func(*SnapshotEngine)

// WithCollector adds a custom collector to the engine.
func WithCollector(c collector.Collector) Option {
	return func(e *SnapshotEngine) {
		_ = e.RegisterCollector(c)
	}
}

// SnapshotEngine coordinates collectors, normalization, hashing, and persistence.
type SnapshotEngine struct {
	mu         sync.RWMutex
	store      store.Store
	collectors map[string]collector.Collector
}

// New creates a new SnapshotEngine with the given store.
// If no collectors are registered via opts, default collectors (OS, Runtime, Env, Git) are added.
func New(s store.Store, opts ...Option) (*SnapshotEngine, error) {
	if s == nil {
		return nil, errors.New(errors.CodeInvalidInput, "store cannot be nil")
	}

	eng := &SnapshotEngine{
		store:      s,
		collectors: make(map[string]collector.Collector),
	}

	// Register standard default collectors
	_ = eng.RegisterCollector(collector.NewOSCollector())
	_ = eng.RegisterCollector(collector.NewRuntimeCollector())
	_ = eng.RegisterCollector(collector.NewEnvCollector(collector.EnvConfig{Enabled: false}))
	_ = eng.RegisterCollector(collector.NewGitCollector(""))

	for _, opt := range opts {
		opt(eng)
	}

	return eng, nil
}

// RegisterCollector registers a collector with the engine.
// Replaces any existing collector with the same name.
func (e *SnapshotEngine) RegisterCollector(c collector.Collector) error {
	if c == nil || c.Name() == "" {
		return errors.New(errors.CodeInvalidInput, "collector or collector name is empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.collectors[c.Name()] = c
	return nil
}

// Capture executes registered collectors, normalizes results, computes content hash,
// and returns a newly constructed snapshot.
func (e *SnapshotEngine) Capture(ctx context.Context, source snapshot.Source, opts ...CaptureOption) (*snapshot.Snapshot, error) {
	if source == "" {
		return nil, errors.New(errors.CodeInvalidInput, "snapshot source cannot be empty")
	}

	cfg := CaptureConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	e.mu.RLock()
	// Collect keys in sorted order for deterministic execution
	names := make([]string, 0, len(e.collectors))
	for name := range e.collectors {
		if cfg.Subset != nil {
			if _, ok := cfg.Subset[name]; !ok {
				continue
			}
		}
		names = append(names, name)
	}
	e.mu.RUnlock()

	sort.Strings(names)

	rawCollected := make(map[string]any, len(names))
	for _, name := range names {
		e.mu.RLock()
		col := e.collectors[name]
		e.mu.RUnlock()

		res, err := col.Collect(ctx)
		if err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "collector failed: "+name, err)
		}
		if res.Data != nil {
			rawCollected[name] = res.Data
		}
	}

	// Normalization
	normalizedData := normalizer.NormalizeMap(rawCollected)

	// Deterministic Content Hashing
	contentHash, err := hasher.ComputeContentHash(normalizedData)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "computing content hash", err)
	}

	snap := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC(),
		SchemaVersion: snapshot.SchemaVersion,
		ParentID:      cfg.ParentID,
		ContentHash:   contentHash,
		Source:        source,
		Data:          normalizedData,
		Labels:        cfg.Labels,
	}

	return snap, nil
}

// Store persists a snapshot to the underlying store.
func (e *SnapshotEngine) Store(ctx context.Context, snap *snapshot.Snapshot) error {
	if snap == nil {
		return errors.New(errors.CodeInvalidInput, "cannot store nil snapshot")
	}

	// Verify content hash before storing (integrity check)
	valid, err := hasher.VerifyContentHash(snap.Data, snap.ContentHash)
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "verifying content hash before store", err)
	}
	if !valid {
		return errors.New(errors.CodeInvalidInput, "snapshot content hash does not match data payload")
	}

	return e.store.Store(ctx, snap)
}

// Load retrieves a snapshot by ID from the store.
func (e *SnapshotEngine) Load(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error) {
	return e.store.Load(ctx, id)
}

// List queries snapshots based on the provided filter.
func (e *SnapshotEngine) List(ctx context.Context, filter store.Filter) ([]*snapshot.Snapshot, error) {
	return e.store.List(ctx, filter)
}

// ListMetadata queries lightweight snapshot summaries matching the filter.
func (e *SnapshotEngine) ListMetadata(ctx context.Context, filter store.Filter) ([]store.Metadata, error) {
	return e.store.ListMetadata(ctx, filter)
}

// Delete removes a snapshot by ID from the store.
func (e *SnapshotEngine) Delete(ctx context.Context, id snapshot.ID) error {
	return e.store.Delete(ctx, id)
}

// Compare compares two snapshots and returns their structural diff.
func (e *SnapshotEngine) Compare(a, b *snapshot.Snapshot) (*Comparison, error) {
	return Compare(a, b)
}
