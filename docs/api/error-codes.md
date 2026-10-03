# Error Codes Reference

Repro uses stable, machine-readable error codes on `*errors.ReproError`. Codes are strictly preserved for backward compatibility and mapped to standardized CLI exit statuses.

---

## Code Reference Table

| Code String | Constant | CLI Exit Code | Trigger Condition |
| :--- | :--- | :--- | :--- |
| `REPRO_UNKNOWN` | `CodeUnknown` | `1` | Fallback when no specific error code applies |
| `REPRO_INVALID_INPUT` | `CodeInvalidInput` | `2` | Caller input fails validation (invalid flags, missing arguments, invalid analyzer) |
| `REPRO_NOT_FOUND` | `CodeNotFound` | `3` | Requested snapshot, event, or file anchor does not exist |
| `REPRO_STORAGE_FAILURE` | `CodeStorageFailure` | `4` | Persistence layer failure, I/O corruption, or disk write error |
| `REPRO_INTERNAL` | `CodeInternal` | `1` | Unexpected internal panic or invariant breakdown |

---

## Error Structure in Go

The error model is defined in `src/core/errors/errors.go`:

```go
type ReproError struct {
    Code     Code            // Machine-readable code
    Message  string          // Human-readable message
    Details  map[string]any  // Contextual key-value pairs
    Cause    error           // Underlying wrapped error
    Recovery string          // Recommended user action
}
```

---

## Error Handling Pattern

```go
package main

import (
	"fmt"

	"github.com/BaimPriyatna/repro/src/core/errors"
)

func CheckError(err error) {
	if err == nil {
		return
	}

	if errors.Is(err, errors.CodeNotFound) {
		fmt.Println("Resource was not found.")
		return
	}

	if re, ok := err.(*errors.ReproError); ok {
		fmt.Printf("[%s] %s\n", re.Code, re.Message)
		if re.Recovery != "" {
			fmt.Printf("Action: %s\n", re.Recovery)
		}
		return
	}

	fmt.Printf("Unexpected error: %v\n", err)
}
```
