# Root Cause & Operations Commands

Commands for diagnosing root causes, rendering explanations, detecting lifecycle issues, analyzing blast radius, and merging configuration files.

---

## `repro why-broken` {#why-broken}

Perform evidence-backed root cause analysis between two snapshots. Automatically queries events recorded between the snapshot timestamps and runs the WhyBroken engine. When `--format human` is used (default), the full ExplainDiff and HumanReadable pipeline is executed automatically.

```bash
repro why-broken [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--from` | string | `""` | Baseline (working) snapshot ID. Defaults to second-latest snapshot |
| `--to` | string | `""` | Broken (current) snapshot ID. Defaults to latest snapshot |
| `--graph` | string | `""` | Path to graph JSON file for dependency-aware analysis |
| `--subject` | string | `""` | Narrow analysis to a specific entity ID |

When both `--from` and `--to` are omitted, Repro automatically picks the two most recent snapshots.

### Examples

Run with the two most recent snapshots (automatic):
```bash
repro why-broken
```

Run between specific snapshots:
```bash
repro why-broken --from snap_01abc --to snap_01xyz
```

Narrow to a specific entity:
```bash
repro why-broken --from snap_01abc --to snap_01xyz --subject "openssl"
```

Include dependency graph for richer analysis:
```bash
repro why-broken --from snap_01abc --to snap_01xyz --graph ./my-graph.json
```

Save JSON result for later use:
```bash
repro why-broken --format json -o result.json
```

### Output (human format)

```text
Root Cause Analysis Report

[HIGH] entity_modified — openssl
  openssl.version changed from 3.0.2 to 3.2.0
  Evidence: snapshot diff snap_01abc -> snap_01xyz
  Impact: linked to 3 dependent packages via repair-map

Conclusion:
  OpenSSL major version update detected. ABI-breaking changes may affect
  native bindings. Consider running 'npm rebuild' or rebuilding native deps.
```

---

## `repro explain-diff` {#explain-diff}

Produce a structured explanation of root cause findings, citing all diagnostic evidence. Can accept a pre-computed `AnalysisResult` JSON file or re-run WhyBroken from snapshot IDs.

```bash
repro explain-diff [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--result` | string | `""` | Path to a pre-computed `AnalysisResult` JSON file |
| `--from` | string | `""` | Baseline snapshot ID |
| `--to` | string | `""` | Broken snapshot ID |
| `--graph` | string | `""` | Path to graph JSON file |
| `--subject` | string | `""` | Entity ID to filter analysis to |
| `--context` | string | `""` | Situational context string to embed in the explanation |

### Examples

Use a previously saved analysis result:
```bash
repro explain-diff --result result.json
```

Re-run and explain from snapshot IDs:
```bash
repro explain-diff --from snap_01abc --to snap_01xyz
```

Add situational context:
```bash
repro explain-diff --from snap_01abc --to snap_01xyz --context "CI failed after merging PR #42"
```

### Output (human format)

```text
Explanation ID: expl_01hxyz...
Context:        CI failed after merging PR #42

Finding #1 [HIGH / confidence: 0.91]:
  entity_modified — openssl
  Evidence: snapshot diff snap_01abc -> snap_01xyz
    openssl.version: 3.0.2 -> 3.2.0
  Interpretation: Major version bump detected. ABI incompatibility is likely.

Finding #2 [MEDIUM / confidence: 0.75]:
  entity_modified — node
  Evidence: snapshot diff snap_01abc -> snap_01xyz
    runtime.node.version: v20.11.0 -> v22.0.0
```

---

## `repro human-readable` {#human-readable}

Render diagnostic findings as plain, structured natural language without re-running any analysis logic. Accepts either an `Explanation` JSON or a raw `AnalysisResult` JSON file (in which case it first runs `ExplainDiff` internally).

```bash
repro human-readable [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--result` | string | `""` | Path to an `Explanation` or `AnalysisResult` JSON file |
| `--from` | string | `""` | Baseline snapshot ID |
| `--to` | string | `""` | Broken snapshot ID |
| `--graph` | string | `""` | Path to graph JSON file |
| `--subject` | string | `""` | Entity ID to filter to |
| `--context` | string | `""` | Situational context string |

### Examples

Render from a saved JSON result:
```bash
repro human-readable --result result.json
```

Render from snapshot pair directly:
```bash
repro human-readable --from snap_01abc --to snap_01xyz
```

### Output (human format)

```text
Diagnosis Report
================

Two changes were detected between snap_01abc and snap_01xyz.

1. OpenSSL was upgraded from version 3.0.2 to 3.2.0. This is a major version
   change that may introduce ABI-breaking differences affecting native bindings
   compiled against the previous version.

2. Node.js runtime was upgraded from v20.11.0 to v22.0.0. Packages with native
   add-ons may require a rebuild ('npm rebuild').

Recommended action: Run 'npm rebuild' and re-run the failing test suite.
```

---

## `repro manual-trace` {#manual-trace}

Scan event records in the store and detect repetitive manual command sequences that could be automated. Analyzes the full event timeline or a filtered time window.

```bash
repro manual-trace [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--after` | string | `""` | Filter events after this timestamp (RFC3339 format, e.g. `2026-10-01T00:00:00Z`) |
| `--before` | string | `""` | Filter events before this timestamp (RFC3339 format) |

### Examples

Scan all events for patterns:
```bash
repro manual-trace
```

Scan only events within a specific time window:
```bash
repro manual-trace --after "2026-10-01T00:00:00Z" --before "2026-10-02T23:59:59Z"
```

### Output (human format)

```text
Scanned 47 events across 3 session(s).

Detected 2 repetitive pattern(s):

Pattern #1 (Occurrences: 4):
  - npm install
  - npm run build
  - npm test
  Suggestion: Create a Makefile target 'make ci' to chain these steps.

Pattern #2 (Occurrences: 3):
  - git stash
  - git pull
  - git stash pop
  Suggestion: Consider using 'git pull --autostash'.
```

---

## `repro config-merge` {#config-merge}

Perform a semantic, conflict-aware merge of two configuration files. Supports TOML and JSON formats (auto-detected from file extension; defaults to JSON).

```bash
repro config-merge [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--a` | string | Yes | Path to configuration file A (baseline) |
| `--b` | string | Yes | Path to configuration file B (incoming) |
| `--strict` | bool | No | Abort with non-zero exit if any conflict is detected |

### Examples

Merge two TOML configs and show result:
```bash
repro config-merge --a config.old.toml --b config.new.toml
```

Strict merge — fail if conflicts exist:
```bash
repro config-merge --a config.old.toml --b config.new.toml --strict
```

Output merged result as JSON:
```bash
repro config-merge --a a.json --b b.json --format json
```

### Output (human format)

```text
Merge Status: 12 merged keys, 1 conflicts
Conflicts:
 - database.port (type_mismatch): 5432 vs "5432"
```

---

## `repro impact` {#impact}

Analyze the blast radius of modifying or removing a set of entities by traversing the dependency graph outward.

```bash
repro impact [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--graph` | string | Yes | Path to graph JSON file |
| `--entity` | []string | Yes | Entity ID(s) to analyze (repeatable) |
| `--max-depth` | int | No | Maximum traversal depth (default: `50`) |

### Examples

Analyze impact of changing a single entity:
```bash
repro impact --graph my-graph.json --entity openssl
```

Analyze impact of multiple entities simultaneously:
```bash
repro impact --graph my-graph.json --entity openssl --entity libssl
```

Limit traversal depth:
```bash
repro impact --graph my-graph.json --entity openssl --max-depth 3
```

### Output (human format)

```text
Impact Analysis: openssl

Direct dependents (3):
  - libssl       (library)
  - node-gyp     (tool)
  - native-addon (package)

Transitive dependents (depth ≤ 50):
  - my-app       (application)  [via node-gyp]
  - storybook    (tool)         [via native-addon]

Total affected entities: 5
```

---

## `repro dead-config` {#dead-config}

Detect configuration items present in the dependency graph that are not referenced by any active entity (i.e., configuration orphans or stale keys).

```bash
repro dead-config [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--graph` | string | Yes | Path to graph JSON file |

### Example

```bash
repro dead-config --graph my-graph.json
```

### Output (human format)

```text
Analysis: dead-config (Status: completed)
Findings (2):
  - [LOW] [dead_config] legacy.cache_ttl: key present in graph but unreferenced
  - [LOW] [dead_config] deprecated.proxy_url: declared but no active entity uses it
```

---

## `repro orphan` {#orphan}

Detect entities in the dependency graph that are not owned by or referenced from any other entity.

```bash
repro orphan [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--graph` | string | Yes | Path to graph JSON file |

### Example

```bash
repro orphan --graph my-graph.json
```

### Output (human format)

```text
Analysis: orphan (Status: completed)
Findings (2):
  - [MEDIUM] [orphan_entity] old-auth-plugin: entity has no parent or dependent
  - [MEDIUM] [orphan_entity] unused-helper.sh: not referenced by any relation in graph
```

---

## `repro ghost-file` {#ghost-file}

Analyze the snapshot history to identify entities (files or paths) that existed in earlier snapshots but were dropped and never reappeared — indicative of abandoned files still lingering in build caches.

```bash
repro ghost-file [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--graph` | string | `""` (required) | Path to graph JSON file |
| `--snapshot` | string | `""` | Starting snapshot ID (defaults to latest) |

### Examples

Detect ghost files from the latest snapshot:
```bash
repro ghost-file --graph my-graph.json
```

Detect ghost files starting from a specific snapshot:
```bash
repro ghost-file --graph my-graph.json --snapshot snap_01xyz
```

### Output (human format)

```text
Analysis: ghost-file (Status: completed)
Findings (2):
  - [LOW] [ghost_file] dist/legacy-bundle.js: present in snap_01abc, absent since snap_01def
  - [LOW] [ghost_file] .cache/old-transform.json: dropped 3 snapshots ago, never returned
```
