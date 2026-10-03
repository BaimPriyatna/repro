# Quick Start

This guide walks you through the core workflow of Repro in under 5 minutes: initializing a project, capturing state snapshots, recording events, diffing states, and diagnosing root causes with `why-broken`.

---

## 1. Initialize a Project

Navigate to your project directory and initialize Repro tracking:

```bash
cd /path/to/your/project
repro init
```

Repro detects your Git root (or current directory) and links it with central storage using an anchor pointer.

```text
Project initialized: my-project
Anchor pointer: .repro
Central store: ~/.repro/central/
```

To run in standalone mode without central storage (ideal for CI/CD runners):

```bash
repro init --local
```

---

## 2. Capture Your Baseline Snapshot

Capture the current state of your development environment (OS runtime, tools, git state, configurations):

```bash
repro capture --reason "initial clean setup"
```

Output:
```text
Snapshot captured successfully!
Snapshot ID: snap_a1b2c3d4e5f6
Timestamp:   2026-10-02T14:00:00Z
Entities:    42
Hash:        sha256:7f83b1657ff1...
```

You can list captured snapshots at any time:

```bash
repro snapshot list
```

---

## 3. Record Development Events

Record changes or milestones as they happen:

```bash
repro add event --type "dependency_update" --source "npm" --desc "Upgraded react from 18 to 19"
```

Events are stored as temporal peers to snapshots, establishing timeline context for later analysis.

---

## 4. Detect State Drift & Changes

After modifying configs, installing tools, or when something starts behaving unexpectedly, take another snapshot and compare:

```bash
repro capture --reason "after dependency bump"
repro diff --before snap_a1b2c3d4e5f6 --after latest
```

Repro reports:
- Added entities (new tools, packages, or config keys)
- Removed entities
- Modified entities with attribute-level diffs

To check drift against the initial baseline:

```bash
repro drift
```

---

## 5. Diagnose Root Cause with WhyBroken

When a build or test fails, ask Repro to trace what broke based on prior evidence:

```bash
repro why-broken --subject "node_modules"
```

Output:
```text
[ANALYSIS] WhyBroken Diagnostic Report
Subject: node_modules
Confidence: 0.92 | Severity: HIGH

Evidence Chain:
  [1] SNAPSHOT: snap_b2c3d4e5 (2026-10-02 14:15:00)
      Node version changed from v20.11.0 to v22.0.0
  [2] GRAPH: Depspy
      Package 'native-binding-x' requires Node ABI <= 115
  [3] EVENT: manual_trace
      Binary compilation failed for native addon

Conclusion:
  Incompatible runtime ABI detected after Node.js version update.
  Suggested resolution: Rebuild native dependencies with 'npm rebuild'
```

---

## Next Steps

- Explore [Core Concepts](/concepts/architecture) to understand snapshot hashing and layers.
- Check out the [CLI Overview](/cli/overview) for all available commands and global flags.
- Learn about the [WhyBroken Diagnostic Chain](/concepts/why-broken).
