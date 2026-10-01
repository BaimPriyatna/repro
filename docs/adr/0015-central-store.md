# ADR 0015: Central Store & Project Cemetery

**Status:** Implemented  
**Date:** 2026-10-01  
**Implements:** Multi-project storage and lifecycle tracking

---

## Context

Projects need persistent storage that survives directory moves, renames, and clones while maintaining history across multiple workspaces. A single-project local `.repro/` directory fails when:
- Directory is renamed or moved
- Project is cloned to multiple locations
- CI environments use ephemeral directories
- Developer wants to track multiple projects from one machine

The system needs intelligent resolution without manual re-association.

---

## Decision

Implement **central store** at `~/.repro/store/` with multi-tier anchor chain resolution:

### Storage Layout

```
~/.repro/store/
├── anchors.toml              # index of all projects
└── projects/
    └── <project-uuid>/
        ├── meta.toml          # project metadata
        ├── snapshots/         # snapshot files
        ├── events/            # event files
        └── graph/             # graph data
```

### Anchor Chain (Priority Order)

1. **Local UUID** — `.repro/anchor` file in project directory
2. **Git Initial Commit SHA** — first commit in git history
3. **Git Remote URL** — canonical remote origin
4. **Known Paths** — exact path match in registered paths

### Project Metadata

```toml
id = "uuid"
name = "project-name"
git_initial_commit = "sha1"
git_remote_url = "https://..."
known_paths = ["/path1", "/path2"]
created_at = "2026-10-01T00:00:00Z"
updated_at = "2026-10-01T00:00:00Z"
last_hostname = "machine"
archived = false

[thresholds]
idle_days = 7
dormant_days = 30
abandoned_days = 90
```

### Lifecycle Status

Projects transition automatically based on time since last activity:
- **active** — activity within idle threshold
- **idle** — no activity for idle_days
- **dormant** — no activity for dormant_days
- **abandoned** — no activity for abandoned_days
- **archived** — manually archived

Thresholds are per-project configurable with defaults: 7/30/90 days.

### Standalone Mode

`REPRO_STORE_MODE=standalone` or `--local` flag maintains local `.repro/` behavior for:
- CI/ephemeral environments
- Temporary analysis
- Projects not requiring central tracking

### Operations

- `repro init [dir]` — register or connect project
- `repro add <path>` — add known path to project
- `repro list` — show all projects
- `repro status [dir]` — show project status
- `repro stats [id]` — detailed project statistics
- `repro cemetery` — lifecycle view of all projects
- `repro archive <id>` — mark as archived
- `repro unarchive <id>` — remove archived mark
- `repro forget <id>` — remove from index (data preserved)
- `repro set-threshold <id>` — configure lifecycle thresholds
- `repro migrate-to-central` — migrate local `.repro/` to central store

---

## Consequences

### Positive

- Directory moves/renames preserve full history via git anchors
- Multiple clones share history through git commit SHA
- Per-project lifecycle tracking surfaces abandoned projects
- Non-destructive operations (no auto-deletion)
- Standalone mode keeps CI/ephemeral workflows unchanged

### Negative

- Non-git projects rely on local UUID (vulnerable to directory loss)
- Central store at `~/.repro/` requires persistent home directory
- Git anchor resolution depends on git metadata availability

### Trade-offs

- Explicit `repro init` over auto-init: informed consent vs. convenience
- Hostname mismatch prompts: safety vs. friction
- Index-only `forget`: data preservation vs. storage cleanup

---

## Implementation Notes

- Atomic writes for `anchors.toml` and `meta.toml` via temp files
- Append-only `known_paths` list
- Thread-safe central store operations
- Git probe handles missing git gracefully
- Local anchor file checked before git probes (performance)
- Cemetery view computes status on-demand (no cached status)

---

## Related

- ADR 0002: Core Domain Types
- ADR 0003: Snapshot Engine
- ADR 0004: Event System
- ADR 0013: CLI Integration
- ADR 0014: Shared CLI Flags
