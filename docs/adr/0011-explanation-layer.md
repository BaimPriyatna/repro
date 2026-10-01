# ADR 0011 — Explanation Layer Architecture (Phase 10)

| Field       | Value                                   |
|-------------|-----------------------------------------|
| Status      | Accepted                                |
| Date        | 2026-09-30                              |
| Phase       | 10 — Explanation Layer                  |

---

## Context

Phase 10 (Section 43) implements the Explanation Layer as a **sequential chain**
(Section 28/29/31, Corrections 80.6):

```text
WhyBroken → ExplainDiff → HumanReadable
```

ExplainDiff (Section 28) translates technical differences into understandable
explanations, always citing evidence. HumanReadable (Section 29) converts that
structured explanation into plain language **without modifying** the underlying
diagnosis.

### Architectural Constraints

- **Sequential, not parallel (Section 80.6)**: HumanReadable consumes ExplainDiff
  output, not raw Analysis Results independently.
- **Scoped to WhyBroken (ROADMAP open question)**: Phase 10 follows the narrower
  current spec. Widening ExplainDiff to all analyzers requires an explicit
  architecture change per Section 79 — not silent scope drift.
- **No re-diagnosis (Section 43)**: Presentation modules must not reimplement
  diagnostic logic. Finding types, confidence, and severity are preserved verbatim.
- **Traceability (Phase 10 exit criteria)**: Every ExplainDiff / HumanReadable
  output must trace back to WhyBroken evidence (finding IDs, snapshot/event refs).

---

## Decision

Implement Phase 10 under `src/presentation/`:

### 1. ExplainDiff (`src/presentation/explaindiff/`)

- **Input**: WhyBroken `*coreanalysis.AnalysisResult` (required; analyzer must be
  `"whybroken"`). Optional `WithDiff(*analysis.DiffResult)` and `WithContext(string)`.
- **Output**: `Explanation` with `Title`, `Summary`, `Observed`, `PotentialEffects`,
  per-finding `ExplainedFinding` narratives, and aggregated `EvidenceRef` citations.
- **Behavior**: Formats existing findings and evidence. Does not reclassify or
  invent new diagnostic conclusions.

### 2. HumanReadable (`src/presentation/humanreadable/`)

- **Input**: `*explaindiff.Explanation` only (enforces the sequential chain).
- **Output**: `Diagnosis` with plain-language `Text` plus immutable
  `SourceExplanationID`, `SourceResultID`, `FindingIDs`, and `EvidenceSourceIDs`.
- **Behavior**: Reformats Explanation into Section 28-style prose
  (Observed / Potential effect / Findings / Evidence). Never mutates the
  Explanation or underlying WhyBroken result.

### Chain usage

```go
exp, _ := explaindiff.New(whyBrokenResult).Explain()
diag, _ := humanreadable.New(exp).Render()
// diag.TracesToResult(string(whyBrokenResult.ID)) == true
```

---

## Consequences

### Positive
- Exit criteria met: diagnosis and explanation both cite WhyBroken finding IDs
  and evidence source IDs.
- Section 80.6 chain enforced at the type level (HumanReadable cannot take a raw
  AnalysisResult).
- Narrow WhyBroken scope documented; widening requires a new ADR.
- Race-clean tests cover validation, evidence citation, non-mutation, and the
  full WhyBroken → ExplainDiff → HumanReadable chain.

### Next Steps
- Phase 11 (**ManualTrace**) is optional and reads Event Store directly.
- CLI presentation commands can wrap these packages later.
