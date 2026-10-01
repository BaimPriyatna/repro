# ADR 0007 — Dependency & Usage Graph Architecture (Phase 6)

| Field       | Value                              |
|-------------|------------------------------------|
| Status      | Accepted                           |
| Date        | 2026-09-29                         |
| Phase       | 6 — Dependency & Usage Graph       |

---

## Context

Phase 6 (Section 38) requires building the Dependency & Usage Graph — the central data structure that models how every component in the development environment relates to every other component. This graph is a hard prerequisite for:

- **Phase 7** (Impact & Lifecycle Analysis) — DeadConfig, Orphan, and GhostFile all query the graph.
- **Phase 8** (Root Cause Analysis) — WhyBroken requires the graph as one of its four required inputs: `Snapshot History + Events + Diff + Dependency Graph` (Section 41).

The ordering constraint in Section 40's note is the strongest architectural rule in the spec: **WhyBroken cannot start before RepairMap is solid.** This ADR documents the decisions made to satisfy that constraint.

Core domain types (`Entity`, `Relation`) were already defined in Phase 1 (`src/core/entity`, `src/core/relation`) and are consumed directly here. No new core types are needed.

---

## Decision

We implement Phase 6 in `src/graph/` with four sub-packages:

### 1. **Shared Foundation (`src/graph/`)**

- **`graph.go`**: Core graph types and the `GraphStore` interface.
  - `Node`: wraps `*entity.Entity` with in/out degree metadata.
  - `Edge`: wraps `*relation.Relation` with resolved endpoint nodes.
  - `Filter`: query parameters (FromID, ToID, RelationKind, EntityKind); zero values = no constraint.
  - `GraphStore` interface: `AddEntity`, `AddRelation`, `GetEntity`, `GetRelation`, `ListEntities`, `ListRelations`, `DeleteEntity`, `DeleteRelation`, `EntityCount`, `RelationCount`.

- **`errors.go`**: Graph-specific errors.
  - `ErrEntityNotFound`, `ErrRelationNotFound`, `ErrDuplicateRelation`, `ErrEndpointMissing`, `ErrInvalidEntity`, `ErrInvalidRelation`.

### 2. **Graph Store (`src/graph/store/`)**

`MemStore` — thread-safe, in-memory `GraphStore` implementation.

**Referential integrity rule**: `AddRelation` rejects any relation whose `FromID` or `ToID` does not exist in the store (`ErrEndpointMissing`). This prevents dangling edges.

**Cascade delete**: `DeleteEntity` removes all relations that reference the deleted entity (incoming and outgoing), preventing stale edge data.

**Thread safety**: all reads use `sync.RWMutex` read locks; all writes use write locks. Safe for concurrent use.

**Filtering**: `ListEntities` and `ListRelations` support multi-dimensional filtering via `Filter`. All fields are optional (zero = no constraint). Multiple conditions are ANDed.

```go
// Example: all imports from a specific entity
rels, _ := store.ListRelations(graph.Filter{
    FromID:       entity.ID("app"),
    RelationKind: relation.KindImports,
})
```

### 3. **Depspy Analyzer (`src/graph/depspy/`)**

Detects hidden and undeclared dependencies by comparing two `GraphStore` instances:

- **`declared`**: the explicit declared dependency model (e.g., from a lockfile, manifest).
- **`observed`**: what was actually detected at runtime/build time (e.g., from OS inspection, import graph analysis).

Three finding types:

| Finding Type         | Severity | Description |
|----------------------|----------|-------------|
| `hidden_dependency`  | High     | Entity observed but not declared |
| `undeclared_relation`| Medium   | Relation observed but not declared (no semantic equivalent) |
| `unused_declaration` | Low      | Entity declared but absent from observed and never a dependency target |

**Semantic equivalence check**: two relations are considered semantically equivalent if they share the same `FromID`, `ToID`, and `Kind`, even if their IDs differ. This prevents false positives from ID-generation differences between declared and observed sources.

**Independence**: Depspy operates solely from the two GraphStore inputs. It does NOT depend on RepairMap, BeforeAfter, Drift, Absent, or ChangeMap.

**Output**: `core/analysis.AnalysisResult` (Section 46). Every finding includes at least one `DiagnosticEvidence` item (Rule 7, Section 45).

### 4. **RepairMap (`src/graph/repairmap/`)**

The primary queryable interface over the dependency graph. Wraps a `GraphStore` and exposes traversal operations consumed by Phases 7 and 8.

**Traversal operations**:

| Method | Description |
|--------|-------------|
| `DirectDependencies(id, kind)` | All entities that `id` directly depends on (outgoing edges) |
| `DirectDependents(id, kind)` | All entities that directly depend on `id` (incoming edges) |
| `Reachable(id, kind, maxDepth)` | All entities transitively reachable from `id` (BFS, cycle-safe) |
| `ImpactSet(id, maxDepth)` | All entities potentially affected by a change in `id` (reverse BFS) |
| `ShortestPath(from, to, kind)` | Shortest directed path between two entities (BFS + path reconstruction) |
| `AllRelationsFor(id)` | All relations in which `id` participates (deduped) |
| `IsQueryable()` | Reports whether the graph has sufficient data to answer queries |

**Cycle safety**: `Reachable` and `ImpactSet` maintain a `visited` set. Both also respect a configurable `maxDepth` to prevent runaway traversal.

**ImpactSet for WhyBroken (Section 41)**:
The `ImpactSet` method is the primary entry point for WhyBroken's candidate generation. It follows all incoming relations transitively, collecting everything that potentially depends (directly or indirectly) on the changed entity.

```go
// WhyBroken usage (Phase 8):
rm, _ := repairmap.New(store)
candidates, _ := rm.ImpactSet(changedEntity, 20)
// candidates = all potentially affected components
```

---

## Architecture Validation

### GraphStore Interface (Section 20)

RepairMap is backed by any `GraphStore` implementation. The interface contract ensures future persistent stores (file-backed, SQLite, etc.) are drop-in replacements for MemStore:

```go
// GraphStore is the persistence and traversal interface.
// Implementations: MemStore (Phase 6), FileStore (future).
type GraphStore interface {
    AddEntity(e *entity.Entity) error
    AddRelation(r *relation.Relation) error
    // ... (see graph.go)
}
```

### Phase Ordering (Section 40)

```
Phase 5 (Diff/History) ─┐
Phase 3 (Events)         ├──► Phase 8 (WhyBroken)
Phase 6 (RepairMap) ─────┘
```

RepairMap MUST be populated before WhyBroken can run. `RepairMap.IsQueryable()` provides this gate check.

### Depspy Independence

Depspy implements the same independence principle as Phase 5 analyzers (Section 80.7): it does not depend on any other analyzer's output.

```go
// Depspy: standalone
d, _ := depspy.New(declared, observed)
result, _ := d.Analyze() // no other analyzer required
```

### Evidence Requirement (Rule 7, Section 45)

All Depspy findings include `DiagnosticEvidence`:

```go
Evidence: []coreanalysis.DiagnosticEvidence{
    {
        Kind:     coreanalysis.EvidenceKindRelation,
        SourceID: string(entityID),
        Field:    "entity.kind",
        Value:    string(entity.Kind),
    },
}
```

No finding is emitted without supporting evidence.

---

## Consequences

### Positive

- Complete Phase 6 exit criteria met: the graph is stable and queryable (`IsQueryable()`)
- RepairMap is ready to be consumed by Phase 7 (DeadConfig, Orphan, GhostFile) and Phase 8 (WhyBroken)
- Clean separation: GraphStore (persistence) vs. RepairMap (traversal) vs. Depspy (analysis)
- Thread-safe MemStore enables concurrent graph construction
- Referential integrity prevents dangling edges
- Comprehensive test coverage (50+ tests across all modules)

### Constraints

- RepairMap traversal uses BFS with depth limits to prevent runaway on large/cyclic graphs
- MemStore keeps all data in memory — a persistent store will be needed for large graphs
- Depspy operates on pre-built graphs; it does not perform live dependency detection itself (that is a capture concern)
- `ShortestPath` returns one shortest path, not all shortest paths

### Future Work

- File-backed or SQLite-backed `GraphStore` for persistence across runs
- Graph serialization/deserialization (JSON export)
- Weighted edges for impact scoring
- Configurable traversal strategies (DFS, weighted BFS)
- Live dependency detection capture module feeding into Depspy
