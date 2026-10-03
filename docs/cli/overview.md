# CLI Overview & Global Flags

All `repro` subcommands share a consistent set of persistent flags that can be used with any command.

---

## Usage

```bash
repro [command] [flags]
```

To print the installed version:

```bash
repro version
```

---

## Global Persistent Flags

These flags are available on every subcommand:

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--format` | string | `human` | Output format: `human` (default) or `json` |
| `-o`, `--output` | string | `""` | Write output to a file instead of stdout |
| `--data-dir` | string | `""` | Override the storage data directory |
| `--local` | bool | `false` | Operate in local standalone mode (bypasses central store) |

### Standalone Mode

When `--local` is passed, Repro stores all data inside `.repro/` relative to the current directory and never touches central storage.

You can also set the environment variable `REPRO_STORE_MODE=standalone` or `REPRO_STORE_MODE=local` to activate standalone mode permanently for a shell session.

```bash
REPRO_STORE_MODE=standalone repro capture --reason "CI build"
```

### JSON Output

Every command supports structured JSON output for scripting and automation:

```bash
repro status --format json
repro snapshot list --format json
repro why-broken --format json > result.json
```

### Writing to a File

The `-o` / `--output` flag redirects any command output to a file (creating parent directories as needed):

```bash
repro why-broken -o /tmp/diagnosis.json --format json
```

---

## Available Commands

| Command | Description |
| :--- | :--- |
| [`capture`](/cli/capture) | Capture development environment state |
| [`snapshot`](/cli/capture#snapshot) | Manage and inspect snapshots (list, get, compare) |
| [`diff`](/cli/analysis#diff) | Compute structural diff between two snapshots |
| [`history`](/cli/analysis#history) | Traverse snapshot lineage history |
| [`drift`](/cli/analysis#drift) | Analyze configuration drift from a baseline snapshot |
| [`absent`](/cli/analysis#absent) | Detect entities present before but missing now |
| [`change-map`](/cli/analysis#change-map) | Build temporal map of changes across snapshot history |
| [`why-broken`](/cli/operations#why-broken) | Evidence-backed root cause analysis |
| [`explain-diff`](/cli/operations#explain-diff) | Explain findings citing diagnostic evidence |
| [`human-readable`](/cli/operations#human-readable) | Render diagnostic results as plain language |
| [`manual-trace`](/cli/operations#manual-trace) | Detect repetitive manual command sequences |
| [`config-merge`](/cli/operations#config-merge) | Semantic merge of two configuration files |
| [`deps`](/cli/analysis#deps) | Detect hidden and undeclared dependencies |
| [`repair-map`](/cli/analysis#repair-map) | Query dependency relationships in the repair map |
| [`impact`](/cli/operations#impact) | Analyze blast radius of modified entities |
| [`dead-config`](/cli/operations#dead-config) | Detect unused configuration items |
| [`orphan`](/cli/operations#orphan) | Detect unowned and unreferenced resources |
| [`ghost-file`](/cli/operations#ghost-file) | Identify abandoned files across snapshot history |
| [`init`](/cli/project#init) | Initialize and register current directory |
| [`add`](/cli/project#add) | Add a known path to an initialized project |
| [`list`](/cli/project#list) | List all registered projects |
| [`status`](/cli/project#status) | Show status of the current project |
| [`stats`](/cli/project#stats) | Display statistics for a project |
| [`cemetery`](/cli/project#cemetery) | List dormant/abandoned projects with inactivity metrics |
| [`archive`](/cli/project#archive) | Mark a project as archived |
| [`unarchive`](/cli/project#archive) | Restore an archived project |
| [`forget`](/cli/project#forget) | Remove a project from the central index |
| [`set-threshold`](/cli/project#set-threshold) | Configure inactivity thresholds for a project |
| [`migrate-to-central`](/cli/project#migrate-to-central) | Migrate local data to central store |
| [`version`](/cli/overview) | Print installed Repro version |
