# ADR 0009 — WhyBroken Root Cause Analysis Architecture (Phase 8)

| Field       | Value                                   |
|-------------|-----------------------------------------|
| Status      | Accepted                                |
| Date        | 2026-09-30                              |
| Phase       | 8 — Root Cause Analysis                 |

---

## Context

Phase 8 (Section 41) implements WhyBroken, the primary root-cause diagnostic module. Section 18 defines its conceptual flow and required output categories. Section 80.4 confirms that RepairMap is a required input — not optional.

Required inputs (Section 41):

```text
Snapshot History + Events + Diff + Dependency Graph
```

### Architectural Constraints & Requirements

- **Four inputs, one classifier**: WhyBroken must consume Diff (from a before/after snapshot pair drawn from History), Events (pre-filtered for the relevant time window), and RepairMap — not any subset alone.
- **No time-proximity causality (Section 18)**: Never claim causality solely because two events happened near each other. Time proximity alone never yields `confirmed_change` or `likely_contributor`.
- **Output categories (Section 18)**: Findings must distinguish `confirmed_change` / `likely_contributor` / `possible_contributor` / `unknown`.
- **Evidence contract (Section 46, Section 47, Rule 7)**: Every finding carries `DiagnosticEvidence` tracing to a real snapshot, event, Diff path, or graph edge. Implements `analysis.Analyzer`.
- **Determinism (Section 3.4)**: Same inputs produce the same finding IDs, types, and sort order.

---

## Decision

We implement Phase 8 as `src/analysis/whybroken/`.

### WhyBroken Analyzer (`src/analysis/whybroken/`)

- **Input**: `*repairmap.RepairMap`, before/after `*snapshot.Snapshot` (history pair for Diff), pre-queried `[]*event.Event`, optional failing `entity.ID` via `WithSubject`.
- **Diff**: Uses shared `diff.Diff` / `diff.FilterChanges` (Phase 5) — no private diff implementation.
- **Graph**: Uses shared `RepairMap` traversal (`DirectDependencies`, `DirectDependents`, `ShortestPath`, `ImpactSet`) — no private graph store.
- **Events**: Caller supplies the time-window filter via Event System (`Query` / `EventsInTimeWindow`); WhyBroken does not own event storage.

### Classification Rules

| Finding type | Evidence combination | Confidence |
|---|---|---|
| `confirmed_change` | Diff change maps to a graph entity on the subject's dependency path **and** a matching event exists | `confirmed` |
| `likely_contributor` | Diff change maps to a **direct** dependency/dependent (or Diff+Graph without subject); no event required | `high` |
| `possible_contributor` | Diff maps to a **transitive** neighbor, **or** event+graph without Diff corroboration | `medium` / `low` |
| `unknown` | Event in window with **no** Diff or graph link — reported, never promoted by time alone | `unknown` |

Entity matching is deterministic exact/suffix matching against entity ID and Name in Diff paths and event Subjects — no naming heuristics beyond that.

Findings are sorted by type priority (`confirmed` → `likely` → `possible` → `unknown`) then subject ID for reproducibility.

---

## Consequences

### Positive
- Exit criteria met: every candidate cause has structured evidence tracing to snapshot, event, Diff, or graph edge.
- Hard rule enforced in code and tests: orphan events stay `unknown`; event-only graph-linked candidates stay at most `possible_contributor`.
- Reuses Phase 5 Diff and Phase 6 RepairMap without duplicating infrastructure.
- Full race-clean test coverage for all four categories, determinism, sorting, and Rule 7 evidence.

### Next Steps (Phase 9 / 10)
- Phase 9 (**ConfigMerge**) can proceed independently (depends only on Phase 1).
- Phase 10 (**ExplainDiff → HumanReadable**) depends on WhyBroken output and must not alter the underlying diagnosis.
