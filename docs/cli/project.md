# Project Management Commands

Manage project registration, storage anchors, and development lifecycle tracking.

---

## `repro init` {#init}

Initialize Repro tracking for the current directory. Detects the Git root (or current directory) and anchors it to central storage.

```bash
repro init [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--local` | bool | `false` | Run in local standalone mode without anchoring to central store |
| `--name` | string | `""` | Project name (defaults to the current directory name) |

### Examples

Initialize with central storage (default):
```bash
repro init
```

Initialize in standalone mode — all data stays inside `.repro/` in the current directory:
```bash
repro init --local
```

Initialize with an explicit project name:
```bash
repro init --name my-api-service
```

### Output (human format)

```text
Project initialized: my-api-service
Anchor pointer: .repro
Central store:  ~/.repro/central/
```

---

## `repro add` {#add}

Add a known path (file or directory) to an initialized project's tracked scope.

```bash
repro add [path] [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--path` | string | `""` | Path to add to the project's tracked scope |

The path can also be supplied as a positional argument.

### Examples

Add a specific config file to tracking scope:
```bash
repro add ./config/app.toml
```

Add a directory to tracking scope:
```bash
repro add ./config
```

---

## `repro status` {#status}

Display the tracking and health status of the active project, including anchor resolution, snapshot count, and lifecycle category.

```bash
repro status
```

### Examples

Show status of the current project:
```bash
repro status
```

Show status in JSON format for scripting:
```bash
repro status --format json
```

### Output (human format)

```text
Project:    my-api-service
Status:     ACTIVE
Anchor:     .repro -> ~/.repro/central/projects/my-api-service
Snapshots:  14
Last seen:  2026-10-02T14:32:00Z
```

---

## `repro list` {#list}

List all projects registered in central storage, including their lifecycle status and snapshot counts.

```bash
repro list
```

### Examples

List all registered projects:
```bash
repro list
```

List as JSON for automation:
```bash
repro list --format json
```

### Output (human format)

```text
NAME                   STATUS     SNAPSHOTS   LAST SEEN
-----------------------------------------------------------------
my-api-service         active     14          2026-10-02 14:32:00
old-frontend           dormant     3          2026-08-15 09:10:00
legacy-worker          abandoned   1          2026-05-01 11:00:00
```

---

## `repro stats` {#stats}

Show aggregated statistics for the current project or all registered projects: total snapshots, events, disk usage, and lifecycle distribution.

```bash
repro stats
```

### Examples

Show stats for the current project:
```bash
repro stats
```

Show stats in JSON:
```bash
repro stats --format json
```

### Output (human format)

```text
Project:      my-api-service
Snapshots:    14
Events:       32
Disk usage:   1.2 MB
Lifecycle:    active

Global summary:
  active:     3
  idle:       1
  dormant:    1
  abandoned:  1
```

---

## `repro archive` & `repro unarchive` {#archive}

Move a project into cold archived status to exclude it from active listings and lifecycle checks. `unarchive` restores an archived project to its previous active lifecycle category.

```bash
repro archive <project-name>
repro unarchive <project-name>
```

### Examples

Archive an idle project:
```bash
repro archive old-frontend
```

Restore an archived project:
```bash
repro unarchive old-frontend
```

### Output (human format)

```text
Project 'old-frontend' archived successfully.
```

---

## `repro cemetery` {#cemetery}

List all projects that have exceeded inactivity thresholds and are categorized as `dormant` or `abandoned`, along with their last-seen timestamps and inactivity duration.

```bash
repro cemetery
```

### Examples

View the cemetery list:
```bash
repro cemetery
```

View as JSON:
```bash
repro cemetery --format json
```

### Output (human format)

```text
CEMETERY — Inactive Projects

NAME               STATUS      LAST SEEN                  INACTIVE FOR
------------------------------------------------------------------------
legacy-worker      abandoned   2026-05-01T11:00:00Z       155 days
old-frontend       dormant     2026-08-15T09:10:00Z        49 days
```

---

## `repro forget` {#forget}

Permanently remove a project from the central index. This does **not** delete snapshot data from disk — only the project registration entry is removed.

```bash
repro forget <project-name>
```

### Examples

Remove a project from the central index:
```bash
repro forget legacy-worker
```

### Output (human format)

```text
Project 'legacy-worker' removed from central index.
Snapshot data at ~/.repro/central/projects/legacy-worker remains on disk.
```

---

## `repro set-threshold` {#set-threshold}

Configure inactivity duration thresholds for automatic lifecycle categorization. Projects that exceed these thresholds are automatically promoted to the next lifecycle category on the next `repro status` or `repro list` check.

```bash
repro set-threshold [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--idle` | string | `"7d"` | Duration before a project is categorized as `idle` |
| `--dormant` | string | `"30d"` | Duration before a project is categorized as `dormant` |
| `--abandoned` | string | `"90d"` | Duration before a project is categorized as `abandoned` |

Duration values accept `d` (days), `h` (hours), e.g. `14d`, `48h`.

### Examples

Set default thresholds:
```bash
repro set-threshold --idle 7d --dormant 30d --abandoned 90d
```

Use stricter thresholds for an active team:
```bash
repro set-threshold --idle 2d --dormant 14d --abandoned 60d
```

### Output (human format)

```text
Thresholds updated:
  idle:      7d
  dormant:   30d
  abandoned: 90d
```

---

## `repro migrate-to-central` {#migrate-to-central}

Migrate a project that was initialized in standalone (`--local`) mode into central multi-project storage. Repro moves the `.repro/` data directory and registers an anchor pointer in `~/.repro/central/`.

```bash
repro migrate-to-central
```

### Examples

Migrate the current local project to central storage:
```bash
repro migrate-to-central
```

Verify the migration succeeded:
```bash
repro status
```

### Output (human format)

```text
Migrating local project to central store...
  Source:      .repro/
  Destination: ~/.repro/central/projects/my-api-service/
  Snapshots:   14 transferred
  Events:      32 transferred

Migration complete. Anchor registered.
```
