# ADR 0003 — Snapshot Engine Architecture (Phase 2)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-28                |
| Phase       | 2 — Snapshot Engine       |

---

## Context

Phase 2 (Section 35) requires implementing the central Snapshot Engine:
collectors, normalization, serialization, content hashing, snapshot IDs,
schema versioning, storage, retrieval, listing, deletion, metadata, and
diffing/comparison.

The Snapshot Engine is the core foundation consumed by all capture modules in
Phase 4 (Repro, TimeCapsule, Watchdog). Rule 2 (Section 45) strictly forbids
individual modules from defining their own snapshot format.

Required capability surface: `capture()`, `store()`, `load()`, `list()`, `compare()`.

Exit criteria: a snapshot can be created, hashed, stored, reloaded byte-identical,
and diffed against itself with zero drift.

---

## Decision

We implement the Snapshot Engine under `src/engine/` partitioned into specialized,
loosely-coupled packages:

1. **`src/engine/collector`**
   - Defines the pluggable `Collector` interface (`Name()`, `Collect(ctx)`).
   - Provides initial built-in collectors:
     - `OSCollector`: captures host OS and architecture (compile-time constants).
     - `RuntimeCollector`: captures Go runtime version.
     - `EnvCollector`: privacy-first (disabled by default, glob deny-list redaction).
     - `GitCollector`: captures HEAD commit, branch, dirty state via git sub-commands.

2. **`src/engine/normalizer`**
   - Implements deterministic recursive data normalization.
   - Normalizes map key order, custom structs, and numeric types into canonical JSON structures.
   - Eliminates drift caused by map iteration order or struct-to-generic unmarshaling roundtrips.

3. **`src/engine/hasher`**
   - Implements deterministic SHA-256 content hashing (`ComputeContentHash`, `VerifyContentHash`).
   - Hashes canonical JSON representation of normalized data payloads.

4. **`src/engine/store`**
   - Defines the `Store` interface (`Store`, `Load`, `List`, `ListMetadata`, `Delete`, `Exists`).
   - Supports metadata-only listing (`MetadataFromSnapshot`) for efficient scanning without deserializing full payloads.
   - Enforces immutability: returns error if a snapshot ID already exists (Rule 6).
   - Detects silent corruption on load via content hash verification (Section 49).
   - Provides two implementations:
     - `FileStore`: filesystem-backed persistence with atomic temp-file-and-rename writes.
     - `MemStore`: thread-safe in-memory store for isolated, high-speed testing.

5. **`src/engine`**
   - Unified `SnapshotEngine` bringing together collectors, normalizer, hasher, and store.
   - Implements `Capture`, `Store`, `Load`, `List`, `ListMetadata`, `Delete`, and `Compare`.
   - `Compare(a, b)`: structural diff identifying added, removed, modified, and unchanged keys. Comparing a snapshot against its reloaded self produces zero drift (`Identical == true`).

---

## Consequences

### Positive

- Satisfies all exit criteria of Phase 2.
- Pluggable collector model allows easily adding future collectors in Phase 4 without touching the engine core.
- Storage layer is decoupled via interface, enabling future pluggable backends (Section 49).
- Privacy constraints (Section 50) enforced by default in `EnvCollector`.
- Atomic writes guarantee crash-resilience against partial snapshot files.

### Constraints

- All collector data payloads must be JSON-serializable.
- Modifying a persisted snapshot file manually triggers corruption detection on load.
