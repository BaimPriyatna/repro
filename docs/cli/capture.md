# Capture & Snapshot Commands

Commands for capturing environment state and managing snapshot records.

---

## `repro capture` {#capture}

Capture the current state of the development environment (OS, runtime tools, git state, configuration files).

```bash
repro capture [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--mode` | string | `oneshot` | Capture mode: `oneshot`, `periodic`, or `watch` |
| `--reason` | string | `""` | Human-readable reason for this capture |
| `--parent` | string | `""` | Parent snapshot ID to link to |
| `--collector` | []string | `nil` | Restrict to specific collector names (repeatable) |
| `--label` | []string | `nil` | Attach labels as `key=value` pairs (repeatable) |
| `--emit-event` | bool | `false` | Emit an event record immediately after capture |

### Capture Modes

| Mode | Description |
| :--- | :--- |
| `oneshot` | Capture once immediately and exit (default) |
| `periodic` | TimeCapsule mode: capture at regular interval (10 min) |
| `watch` | Watchdog mode: report a config-change event and optionally snapshot |

### Examples

Capture current state with a reason:
```bash
repro capture --reason "before node upgrade"
```

Capture and attach labels for filtering later:
```bash
repro capture --reason "pre-deploy" --label "env=staging" --label "team=backend"
```

Capture and link to a parent snapshot to build a lineage chain:
```bash
repro capture --parent snap_01abc123 --reason "post-migration"
```

Capture only specific collectors:
```bash
repro capture --collector os --collector git
```

Capture and emit a corresponding event record:
```bash
repro capture --reason "weekly baseline" --emit-event
```

### Output (human format)

```text
Snapshot ID: snap_01hxyz...
Timestamp:   2026-10-02 14:00:00 UTC
Source:      manual
Hash:        7f83b165
```

---

## `repro snapshot` {#snapshot}

Manage and inspect snapshot records. Accepts an `--action` flag or a positional action argument.

```bash
repro snapshot [action] [flags]
repro snapshot --action <action> [flags]
```

### Actions

| Action | Description |
| :--- | :--- |
| `list` | List all stored snapshots (default) |
| `get` | Retrieve a specific snapshot by ID |
| `capture` | Capture a new snapshot via the engine directly |
| `compare` | Compare two snapshots and show hash-level differences |

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--action` | string | `list` | Action to perform |
| `--snapshot` | string | `""` | Snapshot ID (used by `get`) |
| `--from` | string | `""` | Source snapshot ID (used by `compare`) |
| `--to` | string | `""` | Target snapshot ID (used by `compare`) |
| `--parent` | string | `""` | Parent snapshot ID (used by `capture`) |
| `--max-depth` | int | `0` | Maximum number of snapshots to list (0 = all) |
| `--label` | []string | `nil` | Labels as `key=value` (used by `capture`) |

### Examples

List all snapshots in tabular format:
```bash
repro snapshot list
repro snapshot           # same as list (default action)
```

Retrieve details of a specific snapshot:
```bash
repro snapshot get --snapshot snap_01hxyz
# Or with positional argument:
repro snapshot get snap_01hxyz
```

Compare two snapshots at the hash level:
```bash
repro snapshot compare --from snap_01abc --to snap_01xyz
# Or with positional arguments:
repro snapshot compare snap_01abc snap_01xyz
```

List the latest 10 snapshots:
```bash
repro snapshot list --max-depth 10
```

### Output: `snapshot list` (human format)

```text
ID                                     TIMESTAMP              SOURCE       HASH
------------------------------------------------------------------------------------
snap_01hxyz1234abcd...                 2026-10-02 14:00:00    manual       7f83b165
snap_01hxyz1234abce...                 2026-10-02 14:30:00    manual       a1b2c3d4
```

### Output: `snapshot compare` (human format)

```text
Comparing snap_01abc -> snap_01xyz
Identical: false
Differences (3):
  - runtime.go.version
  - os.hostname
  - git.commit
```
