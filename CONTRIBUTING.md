# Contributing

By participating, you agree to uphold our [Code of Conduct](./CODE_OF_CONDUCT.md).

## Reporting issues

Use the GitHub issue templates:

- **Bug report** — Go version, OS, minimal reproduction, expected vs actual
- **Feature request** — problem, proposed API/CLI shape, alternatives

Security issues: follow [SECURITY.md](./SECURITY.md) (private advisory), not a
public bug report.

## Pull requests

PRs use [`.github/PULL_REQUEST_TEMPLATE.md`](./.github/PULL_REQUEST_TEMPLATE.md).
Before requesting review, make sure the checklist items that apply to your
change are checked — especially tests, `CHANGELOG.md`, and error-code docs when
codes change.

```bash
go mod tidy
make fmt
make vet
make test
make lint   # requires golangci-lint
```

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | ≥ 1.24 | Matches `go.mod` and CI |
| make | any | Wrapper around common `go` commands |
| golangci-lint | latest | Required for `make lint` / CI lint job |

## Setup

```bash
git clone https://github.com/BaimPriyatna/repro.git
cd repro
go mod tidy
make build
./bin/repro --help
```

## Common commands

```bash
make build      # compile ./bin/repro
make test       # full suite with race detector
make test-v     # verbose tests
make cover      # coverage HTML report
make lint       # golangci-lint
make fmt        # gofmt -w -s
make vet        # go vet
make tidy       # go mod tidy + verify
make clean      # remove ./bin and coverage.out
```

## Coding conventions

- **Errors** — use `src/core/errors.New` / `errors.Wrap` with a stable `Code`.
  Do not silently discard data. Document new codes in
  [ERROR-CODES.md](./ERROR-CODES.md).
- **Tests** — every package should have `_test.go` coverage for normal input,
  empty/malformed input, edge cases, and error paths. Prefer `go test -race`.
- **Determinism** — no hidden state or uncontrolled randomness in analysis
  output paths.
- **Privacy** — default to not capturing env vars or secrets.
- **Layers** — follow [ARCHITECTURE.md](./ARCHITECTURE.md). Lower layers must
  not import higher ones (`src/core` never imports analysis/presentation/CLI).
- **Snapshots** — all capture modules must use the shared Snapshot Engine; do
  not invent a parallel snapshot format.
- **Explanation** — presentation packages narrate evidence; they must not
  reimplement diagnostic logic.

## Architectural Decision Records

Significant design choices live in [`docs/adr/`](./docs/adr/). New ADRs should
follow the format of existing files (context, decision, consequences).

| ADR | Title |
| --- | --- |
| 0001 | Language & Runtime: Go |
| 0002 | Core Domain Type Contracts |
| 0003 | Snapshot Engine Architecture |
| 0004 | Event System Architecture |
| 0005 | Capture Modules Architecture |
| 0006 | Diff & History Analysis |
| 0007 | Dependency & Usage Graph |
| 0008 | Impact & Lifecycle Analysis |
| 0009 | WhyBroken Root Cause Analysis |
| 0010 | ConfigMerge Configuration Operations |
| 0011 | Explanation Layer |
| 0012 | ManualTrace Workflow Intelligence |
| 0013 | CLI Integration Track |
| 0014 | Shared CLI Flags & Exit Codes |
| 0015 | Central Store & Project Cemetery |

## Versioning

See [`docs/VERSIONING.md`](./docs/VERSIONING.md). User-facing changes belong in
[CHANGELOG.md](./CHANGELOG.md) under `[Unreleased]` until a release is cut.

## Examples and docs

- Library snippets: [EXAMPLES.md](./EXAMPLES.md)
- System design: [ARCHITECTURE.md](./ARCHITECTURE.md)
- Error codes: [ERROR-CODES.md](./ERROR-CODES.md)
