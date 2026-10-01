# ADR 0001 — Language & Runtime: Go

| Field       | Value                        |
|-------------|------------------------------|
| Status      | Accepted                     |
| Date        | 2026-09-27                   |
| Phase       | 0 — Repository Foundation    |

---

## Context

Phase 0 (Section 33 of the spec) requires choosing a language and runtime before
any functionality is implemented. The choice must remain consistent with:

- **Section 52** — CLI-first interface (`repro <command> [options]`)
- **Section 56** — explicit performance requirements (snapshot creation,
  filesystem scanning, content hashing, graph traversal, parallel collectors)
- **Section 3.4** — deterministic analysis (concurrency model must be explicit)
- **Section 78** — one coherent system, not 18 independent tools

---

## Decision

**Go 1.27** is chosen as the primary language and runtime.

### Toolchain

| Concern         | Tool                                   |
|-----------------|----------------------------------------|
| Package manager | Go modules (`go mod`)                  |
| Linting         | `golangci-lint` (`.golangci.yml`)      |
| Formatting      | `gofmt -s` (enforced in CI)            |
| Testing         | `go test -race` (standard library)     |
| Build shortcuts | `Makefile`                             |
| CI              | GitHub Actions                         |

---

## Alternatives Considered

| Language   | Reason not chosen                                                  |
|------------|--------------------------------------------------------------------|
| Python     | Requires interpreter at runtime; slower for filesystem/hash work  |
| TypeScript | Requires Node.js runtime; event-loop ordering complicates Section 3.4 |
| Rust       | Higher implementation complexity; slower development velocity for 18-module scope |

---

## Consequences

### Positive

- Single static binary: no runtime dependency for end users (Section 52).
- Goroutines + channels give explicit, deterministic concurrency for parallel
  collectors (Section 56) without hidden event-loop ordering.
- Struct-based type system maps cleanly to `Snapshot`, `Event`, `AnalysisResult`
  (Sections 5, 6, 46) with enforced contracts (Rule 4, Section 45).
- Standard toolchain (`go vet`, `gofmt`, `go test -race`) requires no external
  ecosystem to verify correctness.

### Constraints

- All future modules must be written in Go.
- CGo should be avoided unless strictly necessary (complicates cross-compilation).
- Generics (Go 1.18+) may be used where they reduce duplication, but clarity
  takes precedence.
