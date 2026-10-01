# Security Policy

## Privacy defaults

Repro is local-first. Core diagnosis does not require network access.

Environment variables and secrets are **not** captured by default. Collectors
that can include sensitive material (for example the env collector) require an
explicit opt-in. Prefer leaving that off in shared or CI environments.

Snapshots and events may still contain paths, package names, and other local
metadata you choose to capture. Treat stored data under your data directory as
sensitive local state.

## Threat boundaries

Repro is designed as an observability and diagnostic library for development
environments. It must not:

- Execute arbitrary shell commands derived from untrusted analysis input
- Silently overwrite or discard stored snapshots or events
- Imply remote monitoring, cloud telemetry, or automatic environment repair

Analysis and explanation modules narrate evidence already present in snapshots,
events, diffs, and the dependency graph. They do not re-run privileged system
actions as part of diagnosis.

## Reporting security vulnerabilities

If you discover a security vulnerability in this project:

1. **DO NOT** open a public GitHub issue.
2. Open a private security advisory on GitHub (repository → Security tab →
   Report a vulnerability), or contact the maintainer via the contact info on
   their GitHub profile ([@BaimPriyatna](https://github.com/BaimPriyatna)).
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Affected versions / commit
   - Suggested fix (if any)

As a solo-maintained project, response time is best-effort rather than a
guaranteed SLA — the timelines below are targets, not contractual commitments.

## Security update policy

Target response times (best-effort):

- **Critical** (RCE, silent data leakage of secrets): patch as soon as possible,
  aimed at within a few days
- **High** (storage corruption that can go undetected, privacy-default bypass):
  patch release within 1–2 weeks
- **Medium / Low**: fixed in the next regular release

There is currently one major line (`1.x`). If a backport policy becomes
necessary, this section will be updated.

## Related documentation

- [ERROR-CODES.md](./ERROR-CODES.md) — stable machine-readable error codes
- [ARCHITECTURE.md](./ARCHITECTURE.md) — system design and layer boundaries
- [CONTRIBUTING.md](./CONTRIBUTING.md) — how to report issues responsibly
