// Package watchdog monitors environment changes, emits events, and may trigger snapshots.
package watchdog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// ChangeType identifies what kind of change was detected.
type ChangeType string

const (
	ChangeTypeFileModified  ChangeType = "file_modified"
	ChangeTypeFileCreated   ChangeType = "file_created"
	ChangeTypeFileDeleted   ChangeType = "file_deleted"
	ChangeTypeEnvChanged    ChangeType = "env_changed"
	ChangeTypeConfigChanged ChangeType = "config_changed"
	ChangeTypePackageChange ChangeType = "package_changed"
	ChangeTypeRuntimeChange ChangeType = "runtime_changed"
	ChangeTypeGitCommit     ChangeType = "git_commit"
	ChangeTypeUnknown       ChangeType = "unknown"
)

// Change represents a detected change in the environment.
type Change struct {
	Type      ChangeType
	Subject   string
	Timestamp time.Time
	Metadata  map[string]any
}

// ChangeHandler is a callback invoked when a change is detected.
// It receives the detected change and can decide whether to trigger a snapshot.
type ChangeHandler func(ctx context.Context, change *Change) (triggerSnapshot bool, err error)

// Watchdog monitors the environment for changes and emits events.
type Watchdog struct {
	cfg *capture.Config

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}

	// Change handlers
	handlers []ChangeHandler

	// Configuration
	autoSnapshot bool
	throttle     time.Duration
	lastSnapshot time.Time
}

// Config holds Watchdog-specific configuration.
type Config struct {
	// Base capture configuration (required)
	Capture *capture.Config

	// AutoSnapshot determines whether to automatically trigger snapshots on changes
	AutoSnapshot bool

	// Throttle is the minimum time between automatic snapshots (prevents snapshot spam)
	Throttle time.Duration

	// Handlers are custom change handlers (optional)
	Handlers []ChangeHandler

	// AutoStart determines whether to start monitoring on New
	AutoStart bool
}

// New creates a new Watchdog with the provided configuration.
func New(cfg *Config) (*Watchdog, error) {
	if cfg == nil || cfg.Capture == nil {
		return nil, capture.ErrEngineRequired
	}

	if err := cfg.Capture.Validate(); err != nil {
		return nil, err
	}

	if cfg.Capture.EventSystem == nil {
		return nil, capture.ErrEventSystemRequired
	}

	wd := &Watchdog{
		cfg:          cfg.Capture,
		stopCh:       make(chan struct{}),
		autoSnapshot: cfg.AutoSnapshot,
		throttle:     cfg.Throttle,
		handlers:     cfg.Handlers,
	}

	if cfg.AutoStart {
		if err := wd.Start(); err != nil {
			return nil, err
		}
	}

	return wd, nil
}

// AddHandler registers a change handler.
func (wd *Watchdog) AddHandler(handler ChangeHandler) {
	wd.mu.Lock()
	defer wd.mu.Unlock()
	wd.handlers = append(wd.handlers, handler)
}

// ReportChange emits an event for the detected change and may trigger a snapshot.
func (wd *Watchdog) ReportChange(ctx context.Context, change *Change) (*capture.Result, error) {
	if change == nil {
		return nil, capture.ErrInvalidInterval // reusing error, should be ErrInvalidInput
	}

	result := &capture.Result{
		Events: make([]*event.Event, 0, 1),
	}

	eventType := mapChangeTypeToEventType(change.Type)
	evt, err := wd.cfg.EventSystem.Emit(
		ctx,
		eventType,
		event.SourceWatchdog,
		change.Subject,
		event.WithMetadata(change.Metadata),
	)
	if err != nil {
		result.Error = err
		return result, err
	}
	result.Events = append(result.Events, evt)

	shouldSnapshot := false

	wd.mu.RLock()
	handlers := wd.handlers
	wd.mu.RUnlock()

	for _, handler := range handlers {
		trigger, err := handler(ctx, change)
		if err != nil {
			// Handler error is non-fatal, continue checking other handlers
			continue
		}
		if trigger {
			shouldSnapshot = true
			break
		}
	}

	if !shouldSnapshot && wd.autoSnapshot {
		wd.mu.RLock()
		lastSnapshot := wd.lastSnapshot
		throttle := wd.throttle
		wd.mu.RUnlock()

		if throttle == 0 || time.Since(lastSnapshot) >= throttle {
			shouldSnapshot = true
		}
	}

	if shouldSnapshot {
		snap, err := wd.cfg.Engine.Capture(
			ctx,
			snapshot.SourceWatchdog,
			wd.cfg.CaptureOptions("")...,
		)
		if err != nil {
			result.Error = err
			return result, err
		}

		// Store the snapshot
		if err := wd.cfg.Engine.Store(ctx, snap); err != nil {
			result.Error = err
			return result, err
		}

		result.Snapshot = snap

		// Update last snapshot time
		wd.mu.Lock()
		wd.lastSnapshot = time.Now()
		wd.mu.Unlock()

		// Emit SNAPSHOT_CREATED event
		snapEvt, err := wd.cfg.EventSystem.Emit(
			ctx,
			event.TypeSnapshotCreated,
			event.SourceWatchdog,
			fmt.Sprintf("snapshot:%s", snap.ID),
			event.WithSnapshot(snap.ID),
			event.WithMetadata(map[string]any{
				"trigger": string(change.Type),
			}),
		)
		if err == nil {
			result.Events = append(result.Events, snapEvt)
		}
	}

	return result, nil
}

// Start begins change monitoring.
// In this basic implementation, Watchdog doesn't actively poll; it relies on
// external systems calling ReportChange. A production implementation would
// integrate with filesystem watchers, git hooks, package manager hooks, etc.
func (wd *Watchdog) Start() error {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if wd.running {
		return capture.ErrAlreadyRunning
	}

	wd.running = true
	return nil
}

// Stop halts change monitoring.
func (wd *Watchdog) Stop() error {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if !wd.running {
		return capture.ErrNotRunning
	}

	wd.running = false
	close(wd.stopCh)
	wd.stopCh = make(chan struct{}) // reset for potential restart

	return nil
}

// IsRunning returns true if the watchdog is active.
func (wd *Watchdog) IsRunning() bool {
	wd.mu.RLock()
	defer wd.mu.RUnlock()
	return wd.running
}

// Close stops the Watchdog and releases resources.
func (wd *Watchdog) Close() error {
	wd.mu.RLock()
	running := wd.running
	wd.mu.RUnlock()

	if running {
		return wd.Stop()
	}
	return nil
}

// mapChangeTypeToEventType converts a ChangeType to an event.Type.
func mapChangeTypeToEventType(ct ChangeType) event.Type {
	switch ct {
	case ChangeTypeFileModified, ChangeTypeFileCreated, ChangeTypeFileDeleted:
		return event.TypeFileChanged
	case ChangeTypeEnvChanged:
		return event.TypeFileChanged // reuse for now
	case ChangeTypeConfigChanged:
		return event.TypeConfigChanged
	case ChangeTypePackageChange:
		return event.TypePackageInstalled // reuse for now
	case ChangeTypeRuntimeChange:
		return event.TypeRuntimeChanged
	case ChangeTypeGitCommit:
		return event.TypeGitCommit
	default:
		return event.TypeFileChanged // fallback
	}
}
