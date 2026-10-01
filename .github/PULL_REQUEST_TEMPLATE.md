## Summary

Briefly describe what this PR does and why.

## Type of change

- [ ] Bug fix
- [ ] New library module / analyzer / collector
- [ ] CLI / config change
- [ ] Docs / examples only
- [ ] Refactor / chore (no behavior change)
- [ ] Release prep (version bump + CHANGELOG)

## Checklist

- [ ] Tests added or updated (normal, edge, and error paths as relevant)
- [ ] `make test` (or `go test -race ./...`) passes locally
- [ ] `make lint` / `make vet` considered for Go changes
- [ ] `ERROR-CODES.md` updated if error codes were added or changed
- [ ] `CHANGELOG.md` `[Unreleased]` (or release section) updated for user-facing changes
- [ ] Architecture layering respected (lower packages do not import higher ones)
- [ ] Privacy defaults preserved (no secrets / env capture unless opt-in)
- [ ] ADR added or updated under `docs/adr/` when the change is a design decision

## Related issues

Closes #
