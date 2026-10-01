---
name: Bug report
about: Something doesn't work as documented — help us reproduce it
title: "[bug] "
labels: bug
---

## Description

A clear summary of what went wrong.

## Environment

- **Repro version / commit:** (e.g. `v1.0.0` or short SHA)
- **Go version:** (e.g. `1.24.0` — `go version`)
- **OS:** (e.g. Windows 11, macOS 14, Ubuntu 22.04)
- **Package / module:** (e.g. `src/engine`, `src/analysis/whybroken`, CLI)

## Minimal reproduction

Steps or a short code snippet that fails. Prefer the smallest example that still shows the bug.

```go
package main

import (
	"context"
	"fmt"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

func main() {
	eng, err := engine.New(store.NewMemStore())
	if err != nil {
		panic(err)
	}
	snap, err := eng.Capture(context.Background(), snapshot.SourceManual)
	fmt.Println(snap, err)
}
```

## Expected behavior

What you expected to happen.

## Actual behavior

What happened instead (include the full error / `ReproError` code if relevant).

## Additional context

Anything else that helps (links, related issues, logs).
