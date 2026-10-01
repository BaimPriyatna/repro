# ADR 0012 — ManualTrace Workflow Intelligence (Phase 11)

| Field       | Value                                   |
|-------------|-----------------------------------------|
| Status      | Accepted                                |
| Date        | 2026-09-30                              |
| Phase       | 11 — Workflow Intelligence              |

---

## Context

Phase 11 (Section 44) implements ManualTrace, an **optional** module for
workflow intelligence. Section 30 defines its purpose: identify repetitive
manual workflows and suggest automation.

### Architectural Constraints

- **Loosely coupled (Section 30)**: ManualTrace is not part of the core
  diagnostic system.
- **Event Store only (Section 80.3)**: Reads activity/event history directly.
  Must NOT consume Analysis Results, Diff, History, Graph, or WhyBroken.
- **Optional (Section 44)**: Remains an optional capability after Event
  infrastructure is mature (Phase 3 satisfied).

---

## Decision

Implement ManualTrace in `src/intelligence/manualtrace/`.

### API

```go
mt := manualtrace.New(opts...)
result, err := mt.Analyze(events)                    // pre-fetched events
result, err := mt.AnalyzeStore(ctx, store, query)    // Event Store path
```

### Detection Model

1. Filter events to workflow types (default: `COMMAND_EXECUTED`,
   `CONFIG_CHANGED`, `FILE_CHANGED`).
2. Sort chronologically; normalize subjects (trim / collapse whitespace).
3. Split into sessions when idle gap exceeds `MaxGap` (default 30m).
4. Mine contiguous subsequences of length `[MinLength, MaxLength]` that occur
   at least `MinOccurrences` times (defaults 2 / 6 / 2).
5. Suppress strict subsequences dominated by longer, equally-or-more frequent
   patterns.
6. Emit a plain-language automation `Suggestion` per pattern, with `EventIDs`
   as evidence for each occurrence.

### Import boundary

Allowed: `src/core/event`, `src/core/errors`, standard library.
Forbidden: `src/analysis/*`, `src/graph/*`, `src/presentation/*`.

---

## Consequences

### Positive
- Exit criteria aligned with Section 30/80.3: Event Store → ManualTrace only.
- Repeating command and mixed config/command workflows are detected with
  evidence-backed suggestions.
- Deterministic output (sorted patterns, stable keys).
- Race-clean tests cover detection, session gaps, store path, and coupling.

### Next Steps
- Phase 12 (**Central Store & Project Cemetery**) is a post-v1 extension.
- CLI `repro manual-trace` can wrap this package later.
