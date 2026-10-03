# Repro — Version Scheme

Repro uses **Semantic Versioning 2.0.0** (`MAJOR.MINOR.PATCH`).

## Rules

| Segment | Increment when... |
| --- | --- |
| MAJOR | a public Go API or CLI contract breaks backward compatibility |
| MINOR | new functionality is added in a backward-compatible manner |
| PATCH | backward-compatible bug fixes only |

## Git tags

Releases are tagged as `vMAJOR.MINOR.PATCH` on the `main` branch.

## Current version

`v1.0.0` — core library modules implemented (capture, engine, events, graph,
analysis, explanation, operations, intelligence). ADRs 0001–0013 document
design decisions. CLI root command only; module subcommands are not wired yet.

## After v1

Backward-compatible additions (new subcommands, optional store backends, new
analyzers that do not break existing APIs or CLI output) are **MINOR** bumps.
Breaking CLI flags or public Go API / output-schema changes are **MAJOR**.

See [CHANGELOG.md](https://github.com/BaimPriyatna/repro/blob/main/CHANGELOG.md) for released history.
