# ADR 0010 — ConfigMerge Configuration Operations (Phase 9)

| Field       | Value                                   |
|-------------|-----------------------------------------|
| Status      | Accepted                                |
| Date        | 2026-09-30                              |
| Phase       | 9 — Configuration Operations            |

---

## Context

Phase 9 (Section 42) implements ConfigMerge. Section 24 defines it as a
**state operation**, not a conventional diagnostic analyzer. Section 80.2
explicitly detaches it from the Capture → Analysis → Explanation pipeline.

Requirements (Section 42):

- semantic comparison
- conflict detection
- deterministic merging
- explicit conflict resolution

### Architectural Constraints

- **Standalone (Section 24 / 80.2)**: Zero dependency on Diff, History, Graph,
  or Analysis modules. Depends only on Phase 1 (`src/core/errors`) and the
  standard library.
- **No silent discard (Rule 6, Section 45)**: Conflicting values must never be
  overwritten or dropped without an explicit conflict report.
- **Determinism (Section 3.4)**: Same inputs produce the same merged map and
  conflict list order.

---

## Decision

Implement ConfigMerge in `src/operations/configmerge/`.

### API

```go
type Config map[string]any

func Merge(a, b Config) (*Result, error)
func MergeStrict(a, b Config) (*Result, error) // errors if any conflicts
```

`Result` contains:

- `Merged` — deterministic union of non-conflicting keys
- `Conflicts` — explicit list of `{Path, ValueA, ValueB, Reason}`

### Merge Rules

1. Key only in A or only in B → take as-is (defensive clone).
2. Both sides, deeply equal → take once.
3. Both sides are nested `map[string]any` → recurse; keep non-conflicting children.
4. Leaf value mismatch or type mismatch → omit from `Merged`, append to `Conflicts`
   with reason `value_mismatch` or `type_mismatch`.
5. Process keys in sorted order; sort conflicts by path.

### Conflict Reasons

| Reason | When |
|--------|------|
| `value_mismatch` | Same path, different values of compatible types |
| `type_mismatch` | Same path, incompatible types (e.g. map vs scalar, int vs string) |

`MergeStrict` returns the conflict report **and** an error when conflicts
exist, so callers that require a clean merge fail loudly.

---

## Consequences

### Positive
- Exit criteria met: ConfigMerge works standalone with no Diff/History/Graph/Analysis imports.
- Conflicts are always explicit; no silent overwrite path exists.
- Nested configs merge partially — non-conflicting siblings survive.
- Race-clean unit tests cover union, identity, conflicts, nesting, determinism, and immutability of inputs.

### Next Steps
- Phase 10 (ExplainDiff → HumanReadable) depends on WhyBroken, not ConfigMerge.
- CLI wiring for `repro config-merge` can follow later; the operations package is the Phase 9 deliverable.
