# ADR 0008 — Impact & Lifecycle Analysis Architecture (Phase 7)

| Field       | Value                                   |
|-------------|-----------------------------------------|
| Status      | Accepted                                |
| Date        | 2026-09-30                              |
| Phase       | 7 — Impact & Lifecycle Analysis         |

---

## Context

Phase 7 (Section 38, Section 40) implements the Impact & Lifecycle Analysis layer, which builds on the Dependency & Usage Graph (Phase 6, RepairMap) and Snapshot History (Phase 5).

This phase consists of four diagnostic and lifecycle analyzers:
1. **Impact** (Section 21) — Analyzes the potential blast radius of a change to components in the environment.
2. **DeadConfig** (Section 22) — Identifies unused configuration items across three explicit categories: `definitely unused`, `probably unused`, and `unknown`.
3. **Orphan** (Section 23) — Detects resources that have no clear relationship or owner, relying strictly on relationship data rather than simplistic naming heuristics.
4. **GhostFile** (Section 25, Section 80.5) — Tracks file origin, usage, duplication, and lifecycle transitions (`created` → `modified` → `used` → `copied` → `unused` → `deleted`) using a **dual-input** architecture that reconciles Snapshot History with the Usage Graph.

### Architectural Constraints & Requirements

- **Shared Usage Graph (Section 26)**: DeadConfig, Orphan, and GhostFile must not independently implement unrelated graph systems. All analyzers consume the shared `RepairMap` / `GraphStore` infrastructure.
- **Impact Modesty (Section 21)**: Impact must identify "potentially affected" components and must not claim certainty or assume that every dependent component is broken.
- **Dual Input for GhostFile (Section 25, Section 80.5)**: GhostFile explicitly requires two input branches: Snapshot History (for temporal timeline reconstruction) and RepairMap (for active usage and reference tracking).
- **Analyzer Contract & Evidence (Section 46, Section 47, Rule 7)**: Every analyzer implements `analysis.Analyzer`, returns `core/analysis.AnalysisResult`, and grounds every finding with `DiagnosticEvidence`.

---

## Decision

We implement Phase 7 in `src/analysis/` across four dedicated packages:

### 1. **Impact Analyzer (`src/analysis/impact/`)**

- **Input**: `*repairmap.RepairMap` and one or more changed `entity.ID` values.
- **Traversal**: Uses `RepairMap.ImpactSet` (reverse BFS traversal) to discover direct and transitive dependents, calculating the dependency distance and path for each.
- **Output**: Produces findings of type `potentially_affected`.
  - Direct dependents (distance = 1) receive `ConfidenceHigh` and `SeverityHigh`.
  - Transitive dependents receive `ConfidenceMedium` or `ConfidenceLow` and proportional severity.
  - Every finding includes `DiagnosticEvidence` referencing the entity and dependency path.

### 2. **DeadConfig Analyzer (`src/analysis/deadconfig/`)**

- **Input**: `*repairmap.RepairMap`.
- **Target Entities**: Configuration entities (`KindConfig`, environment variables, secrets).
- **Classification Logic**:
  - `definitely unused`: Config entity has 0 incoming relations across the entire graph.
  - `probably unused`: Config entity has incoming relations, but all dependents are themselves inactive, deprecated, or orphaned.
  - `unknown`: Config entity has dynamic or wildcard usage patterns where static graph analysis cannot determine reference status.
- **Output**: Findings of type `dead_config_definitely_unused`, `dead_config_probably_unused`, and `dead_config_unknown` with supporting evidence.

### 3. **Orphan Analyzer (`src/analysis/orphan/`)**

- **Input**: `*repairmap.RepairMap`.
- **Relationship Analysis**:
  - `orphan_disconnected`: Entity has 0 edges (no incoming or outgoing relations).
  - `orphan_unreferenced`: Entity has 0 incoming relations (not depended on or used) and has no `owned_by` relation.
  - `orphan_unowned`: Entity is used in the graph but lacks an explicit `owned_by` relation (applicable to key services and resources).
- **Exclusions**: Root services / entrypoint entities (identified via attributes) are exempted from unreferenced orphan status.

### 4. **GhostFile Analyzer (`src/analysis/ghostfile/`)**

- **Input**: Dual inputs — `*repairmap.RepairMap` (Usage Graph) + `*analysis.HistoryChain` (Snapshot History).
- **Lifecycle Mapping**:
  - Evaluates snapshot timeline chronologically to track file appearances (`created`), hash mutations (`modified`), cross-path duplication (`copied`), and disappearances (`deleted`).
  - Cross-references the latest snapshot state with `RepairMap` file entities and their incoming relations.
  - Detects `ghost_file_unused` (file present in environment but unreferenced in graph) and `ghost_file_deleted_with_references` (file deleted in history but still referenced by active components).

---

## Consequences

### Positive
- Exit criteria met: DeadConfig, Orphan, and GhostFile all read from the same shared RepairMap graph without duplicated traversal logic.
- Impact analyzer strictly adheres to the "potentially affected" semantic requirement without false certainty.
- GhostFile successfully merges temporal snapshot history with live dependency graph state.
- Full test coverage with race-safe in-memory stores and comprehensive assertion suites.

### Next Steps (Phase 8)
- Phase 8 (**WhyBroken** — Root Cause Analysis) can now proceed, having all four required inputs available: `Snapshot History + Events + Diff + Dependency Graph`.
