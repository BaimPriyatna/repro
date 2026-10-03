# Diff, History & Analysis Commands

Commands for comparing snapshots, traversing lineage history, detecting drift, and mapping dependency graphs.

---

## `repro diff` {#diff}

Compute a structural, evidence-backed difference between two snapshots using the BeforeAfter analyzer.

```bash
repro diff [from] [to] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--from` | string | `""` | Source (baseline) snapshot ID |
| `--to` | string | `""` | Target (current) snapshot ID |

Both IDs can also be supplied as positional arguments instead of flags.

### Examples

Diff using flags:
```bash
repro diff --from snap_01abc --to snap_01xyz
```

Diff using positional arguments:
```bash
repro diff snap_01abc snap_01xyz
```

Output as JSON for scripting:
```bash
repro diff --from snap_01abc --to snap_01xyz --format json
```

### Output (human format)

```text
Analysis: before-after (Status: completed)
Findings (2):
 - [HIGH] [entity_modified] runtime.go.version: version changed from 1.23.1 to 1.24.0
 - [LOW] [entity_added] pnpm: new tool detected
```

---

## `repro history` {#history}

Traverse the lineage chain of a snapshot by following parent-ID links backwards.

```bash
repro history [snapshot-id] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--snapshot` | string | `""` | Starting snapshot ID (defaults to latest) |
| `--max-depth` | int | `20` | Maximum traversal depth (number of ancestors) |

The snapshot ID can also be supplied as a positional argument.

### Examples

Traverse lineage from the latest snapshot (up to 20 ancestors):
```bash
repro history
```

Traverse from a specific snapshot:
```bash
repro history --snapshot snap_01xyz
# Or positionally:
repro history snap_01xyz
```

Limit traversal depth:
```bash
repro history --max-depth 5
```

### Output (human format)

```text
Lineage for snap_01xyz (depth: 3):
   1. snap_01xyz   2026-10-02 14:30:00    manual
   2. snap_01abc   2026-10-02 14:00:00    manual
   3. snap_01def   2026-10-01 09:00:00    manual
```

---

## `repro drift` {#drift}

Detect gradual configuration drift from a baseline snapshot by analyzing the full lineage history forward from that baseline.

```bash
repro drift [baseline-id] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--from` | string | `""` | Baseline snapshot ID (defaults to oldest snapshot) |

The baseline ID can also be supplied as a positional argument.

### Examples

Detect drift from the oldest recorded snapshot:
```bash
repro drift
```

Detect drift from a specific baseline:
```bash
repro drift --from snap_01abc
# Or positionally:
repro drift snap_01abc
```

### Output (human format)

```text
Analysis: drift (Status: completed)
Findings (1):
 - [MEDIUM] [config_drift] openssl.version: drifted from 3.0.2 across 4 snapshots
```

---

## `repro absent` {#absent}

Detect entities that existed in a baseline snapshot but are no longer present in the target snapshot.

```bash
repro absent [from] [to] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--from` | string | `""` | Baseline snapshot ID |
| `--to` | string | `""` | Target snapshot ID |

Both IDs can also be supplied as positional arguments.

### Examples

```bash
repro absent --from snap_01abc --to snap_01xyz
repro absent snap_01abc snap_01xyz
```

### Output (human format)

```text
Analysis: absent (Status: completed)
Findings (1):
 - [HIGH] [entity_absent] yarn: was present in baseline but missing in target
```

---

## `repro change-map` {#change-map}

Build a temporal map of entity changes across the full snapshot lineage starting from a given snapshot.

```bash
repro change-map [snapshot-id] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--snapshot` | string | `""` | Starting snapshot ID (defaults to latest) |

The snapshot ID can also be supplied as a positional argument.

### Examples

Build change map from the latest snapshot:
```bash
repro change-map
```

Build change map from a specific snapshot:
```bash
repro change-map --snapshot snap_01xyz
repro change-map snap_01xyz
```

### Output (human format)

```text
Analysis: change-map (Status: completed)

Entity Change Timeline:
  openssl.version
    snap_01abc (2026-10-01 09:00:00)  3.0.2
    snap_01def (2026-10-01 14:00:00)  3.2.0  [MODIFIED]

  runtime.node.version
    snap_01abc (2026-10-01 09:00:00)  v20.11.0
    snap_01xyz (2026-10-02 14:00:00)  v22.0.0  [MODIFIED]

  pnpm
    snap_01xyz (2026-10-02 14:00:00)  (added)  [ADDED]
```

---

## `repro deps` {#deps}

Detect hidden and undeclared dependencies by comparing a declared dependency graph against an observed dependency graph.

Both graphs must be provided as JSON files in `{"entities": [...], "relations": [...]}` format.

```bash
repro deps [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--declared` | string | Yes | Path to the declared dependency graph JSON file |
| `--observed` | string | Yes | Path to the observed dependency graph JSON file |

### Example

```bash
repro deps --declared declared-graph.json --observed observed-graph.json
```

### Graph JSON Format

```json
{
  "entities": [
    {"id": "react", "kind": "library", "name": "react"}
  ],
  "relations": [
    {"from": "my-app", "to": "react", "type": "depends_on"}
  ]
}
```

---

## `repro repair-map` {#repair-map}

Query a dependency graph to see direct dependencies and dependents of a specific entity, or inspect the overall graph size.

```bash
repro repair-map [flags]
```

### Flags

| Flag | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `--graph` | string | Yes | Path to the graph JSON file |
| `--subject` | string | No | Entity ID to inspect; omit to show graph summary |

### Examples

Show summary of the entire graph:
```bash
repro repair-map --graph my-graph.json
```

Inspect direct dependencies and dependents of a specific entity:
```bash
repro repair-map --graph my-graph.json --subject react
```

### Output (human format — with `--subject`)

```text
Entity: react
Direct Dependencies (1):
 - js-tokens (library)
Direct Dependents (2):
 - my-app (application)
 - storybook (tool)
```
