# Library API Overview

Repro is designed as a modular Go library in addition to its CLI interface. You can embed Repro directly into your developer platforms, CI agents, or internal diagnostic tooling.

---

## Installation

Add the Repro module to your Go project:

```bash
go get github.com/BaimPriyatna/repro
```

---

## Package Architecture

Repro packages are organized under `src/`:

```text
github.com/BaimPriyatna/repro/
├── src/
│   ├── core/              # Immutable domain entities, snapshot types, error models
│   ├── engine/            # Snapshot engine, collectors, hasher, storage interfaces
│   ├── capture/           # Sensors for OS, environment, and Git state
│   ├── analysis/          # BeforeAfter, Drift, Absent, ChangeMap analyzers
│   ├── graph/             # Depspy, RepairMap dependency graph builders
│   ├── operations/        # Semantic ConfigMerge operations
│   ├── presentation/      # ExplainDiff, HumanReadable narrative formatters
│   └── central/           # Central store, anchor chain resolver
```

---

## Core Guarantees

1. **Deterministic Hashing**: Snapshots with identical contents produce identical SHA-256 hashes across machines.
2. **Immutable Snapshots**: Once stored, a snapshot record cannot be mutated.
3. **Structured Errors**: All public functions return structured `*errors.ReproError` instances with stable, machine-readable codes.
4. **No Generative Guesswork**: Analysis findings only output facts backed by captured `DiagnosticEvidence`.
