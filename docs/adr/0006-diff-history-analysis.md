# ADR 0006 — Diff & History Analysis Architecture (Phase 5)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-29                |
| Phase       | 5 — Diff & History        |

---

## Context

Phase 5 (Section 37) requires implementing diff and history infrastructure plus four foundational analyzers: BeforeAfter, Drift, Absent, and ChangeMap.

Critical architectural constraint (Section 80.7): all four analyzers operate **independently**. Drift does not require BeforeAfter's output, Absent does not require ChangeMap's output. Each analyzer consumes diff/history infrastructure directly and produces a complete AnalysisResult per Section 46.

Exit criteria: all four produce valid AnalysisResult independently, with every finding traceable to evidence (Rule 7, Section 45).

---

## Decision

We implement the analysis layer in `src/analysis/` with shared infrastructure and four independent analyzer modules:

### 1. **Shared Foundation (`src/analysis/`)**

- **`analysis.go`**: Common types and contracts
  - `Analyzer` interface: Name(), Version(), Analyze() → AnalysisResult
  - `SnapshotLoader` interface: Load(), List() abstractions
  - `ChangeType`: Added, Removed, Modified, Unchanged
  - `Change`: Type, Path, OldValue, NewValue
  - `DiffResult`: SnapshotA, SnapshotB, Changes[], Summary
  - `DiffSummary`: Added, Removed, Modified, Unchanged counts
  - `HistoryQuery`: StartID, MaxDepth, IncludeStart
  - `HistoryChain`: Snapshots[] (newest to oldest), Depth, helpers (Newest(), Oldest(), IsEmpty())

- **`errors.go`**: Analysis-specific errors
  - `ErrSnapshotNotFound`, `ErrInvalidInput`, `ErrInsufficientData`, `ErrNoBaseline`, `ErrEmptyHistory`

### 2. **Diff Infrastructure (`src/analysis/diff/`)**

Provides structural snapshot comparison:

- **Core Function**: `Diff(a, b *snapshot.Snapshot) → *DiffResult`
  - Recursive descent through normalized data (maps, slices, primitives)
  - Path tracking with dot notation (e.g., `runtime.go.version`)
  - Type-aware comparison handling maps, slices, and atomic values
  - Deterministic ordering via sorted keys (Section 3.4)

- **Utilities**:
  - `FilterChanges(changes, types...)`: extract specific change types
  - `GetChangesByPrefix(changes, prefix)`: filter by path prefix

- **Diffing Strategy**:
  - Maps: collect all keys, sort deterministically, recurse on each
  - Slices: compare lengths, then element-by-element
  - Primitives: reflect.DeepEqual
  - Type mismatches: treat as modification

### 3. **History Infrastructure (`src/analysis/history/`)**

Provides snapshot chain traversal:

- **Traverser**: walks parent chains
  - `Traverse(query)`: follow ParentID references, depth-limited, cycle-detected
  - `GetAncestor(id, n)`: retrieve N-th ancestor
  - `GetParent(id)`: immediate parent (convenience for N=1)
  - `FindCommonAncestor(idA, idB)`: most recent shared ancestor
  - `GetSnapshotsBetween(newer, older)`: inclusive range on same chain
  - `CountGenerations(newer, older)`: number of parent hops

- **Safety**:
  - Cycle detection prevents infinite loops (defensive)
  - Depth limits prevent runaway traversal
  - Missing parents stop traversal gracefully

### 4. **BeforeAfter Analyzer (`src/analysis/beforeafter/`)**

Purpose: Compare two specific snapshots (Section 14).

- **Input**: two snapshots (A and B)
- **Process**:
  1. Call `diff.Diff(A, B)`
  2. Convert changes to findings (added → `field_added`, removed → `field_removed`, modified → `field_modified`)
  3. Assign severity (Info for added, Medium for removed/modified)
  4. Confidence = Confirmed (direct observation)
  5. Attach evidence: diff results referencing both snapshots

- **Output**: AnalysisResult with findings for each change
- **Independence**: operates standalone, no dependency on other analyzers

### 5. **Drift Analyzer (`src/analysis/drift/`)**

Purpose: Detect gradual deviation from a baseline (Section 15).

- **Input**: baseline snapshot + history traverser
- **Process**:
  1. Traverse history from baseline (depth-limited to 10)
  2. For each snapshot in chain, diff against baseline
  3. Track modified fields as gradual drift
  4. Generate `gradual_drift` findings

- **Output**: AnalysisResult showing accumulated drift
- **Independence**: does NOT require BeforeAfter to run first

### 6. **Absent Analyzer (`src/analysis/absent/`)**

Purpose: Detect entities present before but missing now (Section 16).

- **Input**: before snapshot + after snapshot
- **Process**:
  1. Call `diff.Diff(before, after)`
  2. Filter for removed changes only
  3. Generate `entity_absent` findings with Medium severity

- **Output**: AnalysisResult listing missing entities
- **Independence**: operates standalone

### 7. **ChangeMap Analyzer (`src/analysis/changemap/`)**

Purpose: Map changes across timeline with temporal context (Section 17).

- **Input**: starting snapshot + history traverser
- **Process**:
  1. Traverse history from start (depth-limited to 20)
  2. Compare consecutive snapshot pairs
  3. Map changes with timestamps
  4. Generate `temporal_change` findings connecting snapshots and time

- **Output**: AnalysisResult with timeline of changes
- **Independence**: does NOT require BeforeAfter or ChangeMap output

---

## Architecture Validation

### Analyzer Independence (Section 80.7)

All four analyzers satisfy the independence requirement:

```go
// BeforeAfter
ba := beforeafter.New(snapA, snapB)
result, _ := ba.Analyze() // standalone

// Drift
drift := drift.New(baseline, traverser)
result, _ := drift.Analyze() // standalone

// Absent
absent := absent.New(before, after)
result, _ := absent.Analyze() // standalone

// ChangeMap
cm := changemap.New(start, traverser)
result, _ := cm.Analyze() // standalone
```

No analyzer calls another analyzer. All consume diff/history infrastructure directly.

### AnalysisResult Contract (Section 46)

Every analyzer produces:
- ID: unique result identifier
- Analyzer: machine-readable name
- Version: analyzer version
- Timestamp: analysis time
- Status: OK or Findings
- Findings[]: structured observations
- Evidence[]: artifact references
- RelatedSnapshots[]: input snapshot IDs

### Evidence Requirement (Rule 7)

All findings include evidence:
```go
finding.Evidence = []DiagnosticEvidence{
    {Kind: EvidenceKindSnapshot, SourceID: snapID, Field: path, Value: value},
    {Kind: EvidenceKindDiff, SourceID: diffID, Field: path, ...},
}
```

No finding is emitted without supporting evidence.

### Deterministic Analysis (Section 3.4)

Diff infrastructure ensures deterministic ordering:
- Map keys sorted alphabetically
- Repeated diffs produce identical change lists
- Test verified: multiple runs yield same order

---

## Consequences

### Positive

- Complete Phase 5 exit criteria met
- All four analyzers produce valid AnalysisResult independently
- Shared diff/history infrastructure eliminates code duplication
- Deterministic output enables reproducible analysis
- Comprehensive test coverage (40+ tests across all modules)
- Clean separation: infrastructure (diff, history) vs. analyzers

### Implementation Details

- **Package structure**: clear boundaries between infrastructure and analyzers
- **Test coverage**: diff (12 tests), history (15 tests), beforeafter (13 tests), plus integration
- **Error handling**: structured errors with specific codes
- **Type safety**: strong typing via Go interfaces and value types

### Constraints

- Analyzers require SnapshotLoader for history traversal
- Diff operates on normalized snapshot.Data (map[string]any)
- History traversal limited by MaxDepth to prevent resource exhaustion
- Slice comparison uses simple element-by-element approach (not LCS)

### Future Work

- More sophisticated slice diffing (LCS algorithm for ordered comparison)
- Configurable drift thresholds and severity classification
- Time-series analysis for ChangeMap (rate of change, acceleration)
- Caching for repeated diff operations
- Parallel analyzer execution framework

