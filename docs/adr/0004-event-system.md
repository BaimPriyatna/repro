# ADR 0004 — Event System Architecture (Phase 3)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-28                |
| Phase       | 3 — Event System          |

---

## Context

Phase 3 (Section 36) requires implementing the Event System:
event model, event creation, event storage, event querying, timestamp ordering,
and optional snapshot association.

Events provide temporal context that snapshots alone cannot represent (Section 6).
Crucially, events are **not** children of snapshots (Correction 80.1). An event's
`related_snapshot_id` is an optional reference, not a parent/child relationship;
events can exist with no snapshot at all (e.g. commands run between snapshots).

Exit criteria: events can be queried by time window and type without touching the
Snapshot Store — this independence is what Phase 12's corrected flow
(`Watchdog → Event → Snapshot Engine`) relies on.

---

## Decision

We implement the Event System in `src/core/event/` with zero dependency on the
Snapshot Engine or Snapshot Store:

1. **Event Model & Creation (`event.go`)**
   - Core domain struct: `Event` with stable fields (`ID`, `Timestamp`, `Type`, `Source`, `Subject`, `Metadata`, `RelatedSnapshotID`).
   - `NewID()`: generates cryptographically secure random UUID v4 identifiers.
   - `New(eventType, source, subject, opts...)`: constructor with safe defaults (UUID v4, UTC timestamp, input validation) and functional options (`WithID`, `WithTimestamp`, `WithMetadata`, `WithSnapshot`).
   - Deep copying: `Clone()` and `WithMetadata` defend against external mutation (Rule 6: immutability).
   - Invariant validation: `Validate()` rejects empty IDs, zero timestamps, or empty mandatory attributes.

2. **Querying & Timestamp Ordering (`query.go`)**
   - `EventQuery`: matches against time windows (`After`, `Before`), single `Type` or multi `Types`, exact `Subject` or `SubjectPrefix`, `Source`, `RelatedSnapshotID`, and `HasSnapshot` boolean.
   - Paging: `Limit` and `Offset`.
   - `SortOrder`: `SortAsc` (chronological) and `SortDesc` (reverse chronological, default).
   - Deterministic sorting (`SortByTimestamp`): tie-breaks identical timestamps using unique `ID` strings, guaranteeing 100% deterministic output (Section 3.4).

3. **Storage Abstraction (`store.go`)**
   - `Store` interface: `Record`, `Get`, `Query`, `Count`, `Delete`, `Exists`, `Close`.
   - Rule 6: `Record` returns `CodeStorageFailure` if an event with the same ID already exists (enforcing immutability).
   - Thread-safe implementations:
     - `MemStore` (`mem.go`): in-memory concurrent map store with defensive copying for unit tests and ephemeral runs.
     - `FileStore` (`file.go`): persistent filesystem store rooted at `<baseDir>/events/<id>.json`. Uses atomic temp-file-and-rename writes, input sanitization against path traversal (Section 51), in-memory metadata indexing for fast querying, and silent corruption detection on load (Section 49).

4. **Event System Service (`system.go`)**
   - `System`: unified coordinator connecting sensors, capture modules, and analyzers to event persistence.
   - High-level operations: `Emit`, `Record`, `Get`, `Query`, `Count`, `Delete`, `EventsForSnapshot`, `EventsInTimeWindow`, `EventsByType`, `Latest`.

---

## Consequences

### Positive

- Complete decoupling from Snapshot Store: Event System operates entirely independently, fulfilling Section 12 and Section 80.1.
- Fully satisfied Phase 3 exit criteria verified by comprehensive unit, integration, and race tests.
- High-performance query execution via in-memory indexing in `FileStore`.
- Strict crash-safety with atomic filesystem operations and immutability guarantees.
- Path traversal protection on all event ID lookups.

### Constraints

- Events are immutable once recorded; retention cleanup uses explicit `Delete`.
- Storage directory requires write access for the `events/` subdirectory.
