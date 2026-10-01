# ADR 0013 — CLI Integration Track (Phase 12)

| Field       | Value                     |
|-------------|---------------------------|
| Status      | Accepted                  |
| Date        | 2026-09-30                |
| Phase       | 12 — CLI Integration Track |

---

## Context

v1.0.0 (Phase 0–11) implements all 18 core modules, but `internal/cli` only wires
the empty root command (Section 52). No module — `capture`, `snapshot`, `diff`,
`why-broken`, etc. — is reachable from `repro <command>` yet.

This was deliberately deferred: each Phase 1–11 ADR scoped itself to its module's
internal contract, not its CLI surface, to avoid coupling module implementation
to command-line design before the module set was stable (Rule 3, Section 45).
Now that the module set is stable at v1.0.0, exposing it is the next step, and it
was decided (see project discussion preceding this ADR) that this warrants its
own phase rather than being folded retroactively into each Phase 1–11 checklist,
since it is one coherent piece of work with its own exit criteria (Section 79).

A second, related problem: without a single flag design pass, each subcommand
would likely invent its own name for the same concept (e.g. one command taking
`--snap`, another `--snapshot-id`, another positional). Section 52 requires the
CLI to "prioritize discoverability and consistency" — that only holds if flags
are designed once, shared, and audited before subcommands are written, not
patched for consistency after the fact.

---

## Decision

1. **New phase, not a retroactive checklist item.** Phase 12 (CLI Integration
   Track) is inserted before the previously-drafted Central Store phase, which
   is renumbered to Phase 13. Phase 12 depends only on Phase 0's root command
   and on Phase 1–11 already existing — it adds no new domain logic, only
   `internal/cli` subcommands that call into existing package APIs.

2. **One shared flag set, audited before implementation.** Before any
   subcommand is written, the flag names used by two or more commands for the
   same concept (snapshot selection, time range, output target, output format)
   are unified into one shared flag group, defined once in `internal/cli` and
   imported by every subcommand. A subcommand only defines a flag of its own
   when no shared flag already covers that input. This is the "flag
   simplification pass" tracked in ROADMAP.md Phase 12.

3. **CLI is a thin adapter.** Subcommands call the existing module APIs
   (`engine.SnapshotEngine`, `event.System`, `analysis.*`, `graph.*`, etc.)
   directly and format their existing return types. No subcommand re-derives
   findings, re-classifies confidence/severity, or otherwise duplicates
   diagnostic logic (Rule 2, Section 45) — matching the constraint already
   placed on `HumanReadable` in ADR 0011.

4. **`--format json` is a direct marshal.** `--format json` serializes the
   module's own struct (`AnalysisResult`, `Snapshot`, etc.) with no
   CLI-specific reshaping, so the machine-readable schema is exactly the
   module's existing contract (Section 53).

5. **Central Store's CLI reuses this flag set.** Phase 13 (Central Store &
   Cemetery) does not define its own flag conventions; its subcommands
   (`repro init`, `repro cemetery`, etc.) are added on top of the Phase 12
   command tree and shared flags.

---

## Alternatives Considered

| Alternative | Reason not chosen |
|---|---|
| Add CLI wiring as a checklist item inside each of Phase 1–11's (already-closed) exit criteria | Those phases are already marked complete for v1.0.0; reopening 11 phases' exit criteria to add CLI work is more disruptive than one new phase, and mixes a v2.x concern into closed v1.0.0 phases |
| Let each subcommand define its own flags, standardize later | Contradicts Section 52's consistency requirement; a later standardization pass would be a breaking CLI change (MAJOR per `docs/VERSIONING.md`) that a flag audit now avoids entirely |
| Fold CLI wiring into Phase 13 (Central Store) instead of its own phase | Phase 13's subcommands are project-lifecycle-specific (`init`, `cemetery`, ...); bundling the 18-module command surface into it would make Phase 13's exit criteria conflate two unrelated concerns |

---

## Consequences

### Positive

- Every v1.0.0 module becomes usable from the CLI without touching its
  internal package contract (Rule 4 — no public API changes).
- One flag audit prevents 18+ independent, inconsistent flag surfaces.
- `--format json` gives downstream tooling (including future `AgentProtocolMode`,
  Section 75) a stable, already-tested schema to build on.

### Constraints

- No subcommand may introduce diagnostic logic not already present in its
  underlying module.
- Adding a genuinely new flag concept after Phase 12 ships requires checking
  the shared flag set first, not adding a per-command one-off.
