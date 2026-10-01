# ADR 0014 — Shared CLI Flags & Exit Codes

| Field  | Value                          |
|--------|--------------------------------|
| Status | Accepted                       |
| Date   | 2026-10-01                     |
| Relates| ADR 0013 — CLI Integration     |

---

## Context

ADR 0013 requires one shared flag set before wiring subcommands, so snapshot
selection, time ranges, and output options use one name per concept.

---

## Decision

### Persistent / shared flags

| Flag | Type | Default | Meaning |
|------|------|---------|---------|
| `--format` | `human` \| `json` | `human` | Output mode |
| `-o` / `--output` | path | stdout | Write output to file |
| `--data-dir` | path | config `Storage.DataDir` | Store root override |
| `--local` | bool | false | Standalone local `.repro/` mode (also `REPRO_STORE_MODE=standalone`) |
| `--snapshot` | id | | Single snapshot ID |
| `--from` / `--to` | id | | Snapshot pair |
| `--parent` | id | | Parent snapshot for capture |
| `--max-depth` | int | 0 (unlimited where applicable) | History / impact depth |
| `--after` / `--before` | RFC3339 | | Event time window |
| `--label` | `k=v` (repeatable) | | Snapshot labels |
| `--collector` | name (repeatable) | | Restrict collectors |
| `--reason` | string | | Capture reason |
| `--emit-event` | bool | false | Emit capture event |
| `--graph` | path | | Graph JSON for RepairMap-backed commands |
| `--declared` / `--observed` | path | | Declared vs observed graph JSON (deps) |
| `--entity` | id (repeatable) | | Changed entity IDs (impact) |
| `--subject` | entity id | | WhyBroken subject filter |
| `--context` | string | | ExplainDiff context |
| `--result` | path | | AnalysisResult / Explanation JSON input |
| `--a` / `--b` | path | | Config files for config-merge |
| `--strict` | bool | false | ConfigMerge strict mode |
| `--action` | string | | Sub-operation for `snapshot` (`capture`, `list`, `get`, `compare`) |

### Exit codes (keyed off `ReproError.Code`)

| Code | Exit |
|------|------|
| success | 0 |
| `REPRO_UNKNOWN` / `REPRO_INTERNAL` / other | 1 |
| `REPRO_INVALID_INPUT` | 2 |
| `REPRO_NOT_FOUND` | 3 |
| `REPRO_STORAGE_FAILURE` | 4 |

### Output

- `--format json`: marshal the module return type as-is.
- `--format human` on WhyBroken `AnalysisResult`: ExplainDiff → HumanReadable text.
- Other `AnalysisResult`: list findings (id, type, severity, confidence, subject).
- Other types: compact summaries or JSON-pretty for structured values.

### Capture modes

TimeCapsule and Watchdog are not separate top-level names in the Phase 12 list.
They are reached via `repro capture --mode oneshot|periodic|watch` (default `oneshot`).

### Graph input JSON

```json
{
  "entities": [{"id":"…","kind":"package","name":"…"}],
  "relations": [{"id":"…","kind":"depends_on","from_id":"…","to_id":"…"}]
}
```

---

## Consequences

All Phase 12/13 subcommands import these flags from `internal/cli`; per-command
flags are only added when no shared flag covers the input.
