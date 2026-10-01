# Error Codes

Repro uses stable, machine-readable error codes on `*errors.ReproError`.
**Codes must not be renumbered or reused once published.** Human-readable
messages may evolve without changing the code.

Defined in [`src/core/errors`](./src/core/errors/errors.go).

## Codes

| Code | Constant | When it is returned |
| --- | --- | --- |
| `REPRO_UNKNOWN` | `CodeUnknown` | Fallback when no more specific code applies |
| `REPRO_INVALID_INPUT` | `CodeInvalidInput` | Caller-supplied input fails validation (nil args, wrong analyzer, bad config, …) |
| `REPRO_NOT_FOUND` | `CodeNotFound` | Requested snapshot, event, or other resource does not exist |
| `REPRO_STORAGE_FAILURE` | `CodeStorageFailure` | Persistence-layer failure, including detected silent corruption |
| `REPRO_INTERNAL` | `CodeInternal` | Unexpected internal condition |

## Shape

```go
type ReproError struct {
    Code     Code
    Message  string
    Details  map[string]any
    Cause    error
    Recovery string
}
```

Construct with `errors.New` / `errors.Wrap`. Attach context with
`WithDetails` / `WithRecovery`. Match codes with `errors.Is(err, code)`.

## Stability rules

- Adding a **new** code is a backward-compatible change (document it here and
  in `CHANGELOG.md`).
- Changing the meaning of an existing code, or removing one, is a **breaking**
  change (MAJOR version bump).
- CLI exit-code mapping (when wired) should key off `Code`, not message text.

## Related

- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [SECURITY.md](./SECURITY.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
