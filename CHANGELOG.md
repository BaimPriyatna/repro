# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.6] - 2026-10-02

### Added

- Installation section in `README.md` documenting Go install, Linux/macOS curl installer, Windows PowerShell installer, and manual installation steps

## [1.1.5] - 2026-10-02

### Added

- PowerShell installer script (`install.ps1`) for Windows (amd64/arm64) with SHA-256 verification, user-local installation, and PATH instructions

## [1.1.4] - 2026-10-02

### Added

- POSIX installer script (`install.sh`) supporting Linux and Darwin (amd64/arm64) with SHA-256 verification and automatic PATH notice

## [1.1.3] - 2026-10-02

### Added

- GitHub Actions release workflow (`.github/workflows/release.yml`) for automated builds and releases on version tags

## [1.1.2] - 2026-10-02

### Added

- GoReleaser configuration (`.goreleaser.yml`) for multi-platform builds, packaging, checksum generation, and GitHub Releases

## [1.1.1] - 2026-10-02

### Added

- Release platform matrix definition covering 6 targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64)

## [1.1.0] - 2026-10-02

### Added

- `version` subcommand (`repro version`) and package-level `version` variable in `cmd/repro/main.go` supporting build-time linker flags

## [1.0.0] - 2026-10-01

### Added

- Core domain types: Snapshot, Event, Entity, Relation, AnalysisResult,
  DiagnosticEvidence, structured errors (`src/core/*`)
- Snapshot Engine: collectors (OS, runtime, env, git), normalizer, hasher,
  mem/file stores, capture / store / load / list / compare (`src/engine/*`)
- Event system: mem/file stores, query model, high-level `System` service
- Capture modules: Repro, TimeCapsule, Watchdog (`src/capture/*`)
- Diff & history infrastructure and analyzers: BeforeAfter, Drift, Absent,
  ChangeMap (`src/analysis/*`)
- Dependency & usage graph: Depspy, RepairMap (`src/graph/*`)
- Impact & lifecycle analyzers: Impact, DeadConfig, Orphan, GhostFile
- WhyBroken root-cause analyzer
- ConfigMerge configuration operation (`src/operations/configmerge`)
- Explanation layer: ExplainDiff → HumanReadable (`src/presentation/*`)
- ManualTrace workflow intelligence (`src/intelligence/manualtrace`)
- Complete CLI with all module subcommands: capture, snapshot, diff, history,
  drift, absent, change-map, why-broken, deps, repair-map, impact, dead-config,
  orphan, config-merge, ghost-file, explain-diff, human-readable, manual-trace
  (`internal/cli/`)
- Central store with multi-project support and anchor chain resolution
  (`src/central/*`)
- Project lifecycle tracking: active / idle / dormant / abandoned / archived
  status with configurable thresholds
- Project management commands: init, add, list, status, stats, cemetery,
  archive, unarchive, forget, set-threshold, migrate-to-central
- Shared CLI flags: --format (human/json), -o/--output, --data-dir, --local
- Standalone mode support for CI/ephemeral environments
- Base config (`internal/config`)
- CI workflow (build, `go test -race`, golangci-lint)
- Public documentation suite: `ARCHITECTURE.md`, `CONTRIBUTING.md`,
  `CODE_OF_CONDUCT.md`, `SECURITY.md`, `ERROR-CODES.md`, `EXAMPLES.md`,
  GitHub issue/PR templates
- Apache License 2.0 `LICENSE`
- ADRs 0001–0015 under `docs/adr/`

### Notes

- v1.0.0 is the initial release with full library, CLI, and storage functionality.
