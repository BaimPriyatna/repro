# ADR 0005 — Capture Modules Architecture (Phase 4)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-29                |
| Phase       | 4 — Capture Modules       |

---

## Context

Phase 4 (Section 32) requires implementing the three Layer 1 Capture modules:
**Repro** (one-shot environment capture for reproduction), **TimeCapsule** 
(scheduled/manual historical snapshots), and **Watchdog** (change detection sensor).

These three modules form the entry point to the entire Repro system. The most 
critical architectural rule (Section 45, Rule 2): **all three must call the same 
Snapshot Engine**. No module gets its own snapshot format.

Section 11 establishes the key difference between Repro and TimeCapsule:
- Repro answers: "What is the state needed to reproduce this situation?"
- TimeCapsule answers: "What was the state at this point in time?"

Section 12 establishes the canonical flow for Watchdog:
```
Watchdog → detects change → Event → Snapshot Engine → Snapshot
```

Watchdog is a **sensor/trigger layer**, not an analysis engine. It must NOT 
directly implement Drift, Absent, or other analysis logic (those belong to Layer 2).

Section 80.1 correction: Events come FROM Watchdog (and other sensors), not from 
Snapshots. Events are siblings of Snapshots, not children.

Exit criteria: triggering Watchdog on a change produces an Event, and that Event 
can optionally trigger a new Snapshot through the same engine Repro and TimeCapsule use.

---

## Decision

We implement the capture layer in `src/capture/` with three modules sharing a 
unified configuration and interface:

### 1. **Shared Foundation (`src/capture/`)**

- **`capture.go`**: Common types and abstractions
  - `Engine` interface: abstracts `Capture()`, `Store()`, `Load()` methods from the 
    Snapshot Engine, allowing all three modules to depend on the same interface.
  - `EventSystem` interface: abstracts `Emit()` and `Record()` for event emission 
    (required for Watchdog, optional for Repro/TimeCapsule).
  - `Config`: shared configuration holding references to the Engine and EventSystem, 
    plus common fields (Labels, CollectorNames).
  - `Result`: standardized return type carrying `Snapshot`, `Events[]`, and `Error`.
  - `CaptureOptions()`: builder that generates `engine.CaptureOption` values from 
    Config, ensuring consistent option handling across all modules.

- **`errors.go`**: Capture-specific error definitions
  - `ErrEngineRequired`: returned when Config.Engine is nil
  - `ErrEventSystemRequired`: returned when EventSystem is required but missing
  - `ErrInvalidInterval`: returned for invalid time intervals
  - `ErrAlreadyRunning` / `ErrNotRunning`: lifecycle state errors

- **`capture_test.go`**: Unit tests for shared types with mock implementations of 
  Engine and EventSystem interfaces.

- **`integration_test.go`**: Cross-module integration tests verifying:
  1. All three modules use the shared Snapshot Engine (Architecture Rule 2)
  2. Watchdog follows the canonical flow (Watchdog → Event → Snapshot)
  3. Historical linkage via parent references works correctly

### 2. **Repro (`src/capture/repro/`)**

Purpose: One-shot environment capture for reproduction and diagnosis.

- **Core API**:
  - `New(cfg)`: creates a Repro instance with the shared Config
  - `Capture(ctx, opts...)`: performs capture via the shared Engine, returns Result
  - `Store(ctx, snap)`: convenience wrapper for Engine.Store()
  - `CaptureAndStore(ctx, opts...)`: atomic capture-and-persist operation

- **Functional Options**:
  - `WithLabels(labels)`: attach metadata labels to the snapshot
  - `WithParent(parentID)`: set parent snapshot for historical linkage
  - `WithReason(reason)`: document why the capture was triggered
  - `WithEvent()`: emit a SNAPSHOT_CREATED event after successful capture
  - `WithCollectors(names...)`: restrict capture to specific collectors

- **Behavior**:
  - Source is always `snapshot.SourceRepro`
  - Snapshot is NOT automatically stored; caller controls persistence
  - Event emission is opt-in via `WithEvent()` option
  - Merges module-level Config.Labels with call-specific labels

- **Tests**: 16 test cases covering all options, event emission, shared engine usage

### 3. **TimeCapsule (`src/capture/timecapsule/`)**

Purpose: Maintain historical snapshots of project/environment state via manual or 
periodic capture.

- **Core API**:
  - `New(cfg)`: creates a TimeCapsule with optional AutoStart
  - `Capture(ctx, opts...)`: manual snapshot capture (automatically stored)
  - `Start(interval)`: begin periodic snapshot capture at specified interval
  - `Stop()`: halt periodic capture
  - `IsRunning()`: check if periodic capture is active
  - `LastCaptureID()`: retrieve ID of most recent snapshot
  - `Close()`: cleanup and stop if running

- **Configuration** (`timecapsule.Config`):
  - `Capture`: shared capture.Config (required)
  - `Interval`: time duration for periodic snapshots
  - `AutoStart`: start periodic capture immediately on New()
  - `RetainCount`: maximum snapshots to retain (0 = unlimited, not yet implemented)

- **Functional Options**:
  - `WithLabels(labels)`: attach metadata labels
  - `WithParent(parentID)`: explicit parent reference
  - `WithReason(reason)`: document capture trigger

- **Behavior**:
  - Source is always `snapshot.SourceTimeCapsule`
  - Snapshots are ALWAYS automatically stored (unlike Repro)
  - Automatic parent linkage: each capture links to `LastCaptureID` unless parent 
    explicitly provided
  - Periodic capture runs in background goroutine with configurable interval
  - Throttling prevents snapshot spam in high-frequency scenarios
  - Emits SNAPSHOT_CREATED event for every capture (if EventSystem present)

- **Tests**: 14+ test cases covering manual/periodic capture, auto-start, 
  start/stop lifecycle, parent linkage, throttling

### 4. **Watchdog (`src/capture/watchdog/`)**

Purpose: Monitor environment for changes, emit events, and optionally trigger 
snapshots (sensor/trigger layer, NOT analysis).

- **Core API**:
  - `New(cfg)`: creates Watchdog; requires EventSystem (not optional)
  - `ReportChange(ctx, change)`: primary API for reporting detected changes
  - `AddHandler(handler)`: register custom ChangeHandler callbacks
  - `Start()`: activate monitoring (placeholder for future active polling)
  - `Stop()`: deactivate monitoring
  - `IsRunning()`: check monitoring state
  - `Close()`: cleanup

- **Configuration** (`watchdog.Config`):
  - `Capture`: shared capture.Config with EventSystem required
  - `AutoSnapshot`: whether to automatically trigger snapshots on changes
  - `Throttle`: minimum time between automatic snapshots (prevents spam)
  - `Handlers`: custom ChangeHandler callbacks
  - `AutoStart`: start monitoring immediately on New()

- **Change Types** (`ChangeType`):
  - `ChangeTypeFileModified`, `ChangeTypeFileCreated`, `ChangeTypeFileDeleted`
  - `ChangeTypeEnvChanged`, `ChangeTypeConfigChanged`
  - `ChangeTypePackageChange`, `ChangeTypeRuntimeChange`
  - `ChangeTypeGitCommit`, `ChangeTypeUnknown`

- **ChangeHandler**: `func(ctx, change) (triggerSnapshot bool, error)`
  - Custom handlers can inspect changes and decide whether to trigger snapshots
  - Multiple handlers can be registered; first returning `true` triggers snapshot

- **Canonical Flow** (Section 12):
  1. `ReportChange()` receives Change
  2. Emit Event via EventSystem (Watchdog → Event)
  3. Consult handlers and AutoSnapshot config to decide if snapshot needed
  4. If yes, call shared Engine.Capture() (Event → Snapshot Engine)
  5. Store snapshot and emit SNAPSHOT_CREATED event
  6. Return Result containing Event(s) and optional Snapshot

- **Behavior**:
  - Source is always `snapshot.SourceWatchdog`
  - Events are ALWAYS emitted (EventSystem is required)
  - Snapshot creation is conditional based on AutoSnapshot + Throttle + Handlers
  - Throttling: prevents snapshot creation if `time.Since(lastSnapshot) < Throttle`
  - This implementation is a foundation; production would integrate with filesystem 
    watchers, git hooks, package manager hooks, etc.

- **Tests**: 13+ test cases covering change reporting, event emission, auto-snapshot, 
  throttling, handlers, lifecycle, canonical flow verification

---

## Architecture Validation

### Rule 2 Compliance (Section 45): Shared Snapshot Engine

All three modules satisfy the requirement:

```go
// Repro
snap, err := r.cfg.Engine.Capture(ctx, snapshot.SourceRepro, ...)

// TimeCapsule  
snap, err := tc.cfg.Engine.Capture(ctx, snapshot.SourceTimeCapsule, ...)

// Watchdog
snap, err := wd.cfg.Engine.Capture(ctx, snapshot.SourceWatchdog, ...)
```

The `Engine` interface ensures no module can create snapshots outside the unified 
format. Integration tests explicitly verify all snapshots land in the same Store.

### Watchdog Canonical Flow (Section 12)

Watchdog correctly implements:
```
Watchdog.ReportChange() 
  → EventSystem.Emit(changeEvent)         // Watchdog → Event
  → Engine.Capture() [conditional]        // Event → Snapshot Engine  
  → Engine.Store()
  → EventSystem.Emit(snapshotCreatedEvent)
```

This matches Section 12's specification and Section 80.1's correction that Events 
are not children of Snapshots.

### Separation of Concerns

- **Repro**: one-shot, user-driven, minimal automation
- **TimeCapsule**: time-based, historical, automatic parent linkage
- **Watchdog**: event-driven sensor, does NOT analyze (no Drift/Absent logic)

Each module has a single, focused responsibility.

---

## Consequences

### Positive

- Complete Phase 4 exit criteria: Watchdog change → Event → Snapshot via shared Engine
- Zero format divergence: all snapshots have identical structure regardless of source
- Explicit source tracking: `snapshot.Source` distinguishes Repro/TimeCapsule/Watchdog
- Flexible composition: modules can be used independently or together
- Event flow correctly decouples Events from Snapshots (Section 80.1)
- Comprehensive test coverage: 46+ unit tests + 3 integration tests, all race-clean
- Production-ready lifecycle management (Start/Stop/Close) for long-running modules

### Trade-offs

- Watchdog is a passive foundation: production deployment requires integration with 
  external watchers (filesystem, git, package managers)
- TimeCapsule periodic capture runs in goroutine; context cancellation and graceful 
  shutdown handled via Stop/Close
- Throttling is time-based only; more sophisticated rate limiting (token bucket, etc.) 
  could be added later if needed

### Constraints

- All three modules require a valid `capture.Config` with non-nil `Engine`
- Watchdog additionally requires non-nil `EventSystem` (architectural requirement)
- Modules do not own the Engine or EventSystem lifecycle; caller must manage closure

### Future Work

- Retention policies for TimeCapsule (Config.RetainCount honored)
- Active filesystem/git/package monitoring for Watchdog (currently passive ReportChange)
- Compression for stored snapshots (Engine concern, not module concern)
- Incremental snapshots (Engine feature leveraging parent references)

