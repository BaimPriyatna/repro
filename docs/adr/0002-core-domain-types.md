# ADR 0002 — Core Domain Type Contracts (Phase 1)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-28                |
| Phase       | 1 — Core Domain           |

---

## Context

Phase 1 (Section 34) requires the fundamental domain types to be defined with
stable field contracts before any higher-level module is implemented.

The six types are:

```text
Snapshot
Event
Entity
Relation
AnalysisResult
DiagnosticEvidence
```

Several key design decisions had to be made at this stage to avoid locking in
the wrong shape downstream:

1. **Snapshot.Data is `map[string]any`** — collectors have heterogeneous schemas
   and the storage layer must not drop unknown keys (Rule 6). A typed union would
   require knowing all collector schemas at Phase 1, which contradicts the phased
   approach.

2. **Event.RelatedSnapshotID is `snapshot.ID`, not a hard FK** — Section 6
   explicitly states this is an optional back-reference (Correction 80.1). An
   Event is structurally valid with an empty RelatedSnapshotID.

3. **Confidence ≠ Severity** — Section 46 and Section 48 require these to be
   separate axes. They must not be conflated (a critical finding can have low
   confidence). Using distinct named types in Go enforces this at compile time.

4. **DiagnosticEvidence is a reference, not an embedded payload** — Embedding the
   full artifact would make AnalysisResult unbounded in size. The `SourceID`
   field is a stable pointer; the analysis consumer retrieves the artifact from
   the appropriate store.

---

## Decision

Implement the six core domain types as pure Go structs with no external
dependencies except the standard library and each other, in the following
packages under `src/core/`:

| Package              | Types                                  |
|----------------------|----------------------------------------|
| `snapshot`           | `Snapshot`, `ID`, `Source`            |
| `event`              | `Event`, `ID`, `Type`, `Source`       |
| `entity`             | `Entity`, `ID`, `Kind`                |
| `relation`           | `Relation`, `ID`, `Kind`              |
| `analysis`           | `AnalysisResult`, `Finding`, `DiagnosticEvidence`, `Confidence`, `Severity`, `ResultStatus`, `EvidenceKind` |

The `src/core/errors` package (Phase 0) is already in place and remains
unchanged.

### Import graph (allowed directions only — Section 74)

```text
analysis  → snapshot, event, entity
relation  → entity
event     → snapshot
snapshot  (no core deps)
entity    (no core deps)
```

No Phase 1 package imports from `internal/config`, `analysis`, or any higher
layer. This preserves the `Core → Storage → Analysis` dependency direction
(Section 74).

---

## Alternatives Considered

| Alternative | Reason not chosen |
|---|---|
| Protobuf/generated types | Adds code-gen to CI; types are simple enough that hand-written structs are clearer and more reviewable |
| Interface-based contracts (`Snapshotable`) | Premature abstraction at Phase 1 — no polymorphism is needed until Phase 2/4 |
| Typed Collector payloads in Snapshot.Data | Would require all collector schemas at Phase 1; `map[string]any` is compatible with all future collectors |

---

## Consequences

### Positive

- All downstream modules (Phase 2–11) can import from `src/core/` without
  circular dependencies.
- The `Severity ≠ Confidence` invariant is enforced at compile time via distinct
  named types.
- `DiagnosticEvidence` keeps AnalysisResult small and storage-friendly.
- `Event.RelatedSnapshotID` is optional, matching Correction 80.1 and preventing
  wrong assumption that Events are children of Snapshots.

### Constraints

- `Snapshot.Data` and `Event.Metadata` use `map[string]any`; strongly-typed
  collector payloads are a Phase 2 concern.
- Changing a field name in any of these structs is a breaking change to the
  JSON schema and requires a `SchemaVersion` bump (Section 59).
- Any module that introduces a new `EvidenceKind`, `Confidence` value, or
  `ResultStatus` must update this ADR.
