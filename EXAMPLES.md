# Examples

Library-oriented snippets for Repro v1. The CLI currently exposes only the
root command; full subcommands are planned for a later release. Until then,
call the Go packages directly.

Import path prefix: `github.com/BaimPriyatna/repro`.

---

## Capture and compare two snapshots

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

func main() {
	ctx := context.Background()
	mem := store.NewMemStore()
	eng, err := engine.New(mem)
	if err != nil {
		log.Fatal(err)
	}

	before, err := eng.Capture(ctx, snapshot.SourceManual)
	if err != nil {
		log.Fatal(err)
	}
	if err := eng.Store(ctx, before); err != nil {
		log.Fatal(err)
	}

	after, err := eng.Capture(ctx, snapshot.SourceManual, engine.WithParent(before.ID))
	if err != nil {
		log.Fatal(err)
	}
	if err := eng.Store(ctx, after); err != nil {
		log.Fatal(err)
	}

	cmp, err := eng.Compare(before, after)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("identical=%v changed_keys=%d\n", cmp.Identical, len(cmp.ModifiedKeys))
}
```

Use `store.NewFileStore(dir)` instead of `NewMemStore` for durable snapshots.

---

## BeforeAfter analysis

```go
import (
	"fmt"
	"log"

	"github.com/BaimPriyatna/repro/src/analysis/beforeafter"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func analyze(a, b *snapshot.Snapshot) {
	ba, err := beforeafter.New(a, b)
	if err != nil {
		log.Fatal(err)
	}
	result, err := ba.Analyze()
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range result.Findings {
		fmt.Printf("%s [%s/%s] %s\n", f.ID, f.Severity, f.Confidence, f.Type)
		for _, ev := range f.Evidence {
			fmt.Printf("  evidence: %s %s\n", ev.Kind, ev.SourceID)
		}
	}
}
```

Every finding must carry evidence. See [ERROR-CODES.md](./ERROR-CODES.md) for
how invalid inputs surface as `REPRO_INVALID_INPUT`.

---

## Explanation chain (WhyBroken → ExplainDiff → HumanReadable)

WhyBroken needs a RepairMap, a before/after snapshot pair, and related events.
Once you have an `AnalysisResult` from WhyBroken:

```go
import (
	"fmt"
	"log"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
	"github.com/BaimPriyatna/repro/src/presentation/humanreadable"
)

func explain(result *coreanalysis.AnalysisResult) {
	ed, err := explaindiff.New(result)
	if err != nil {
		log.Fatal(err)
	}
	explanation, err := ed.Explain()
	if err != nil {
		log.Fatal(err)
	}

	hr, err := humanreadable.New(explanation)
	if err != nil {
		log.Fatal(err)
	}
	diagnosis, err := hr.Render()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(diagnosis.Text)
}
```

`ExplainDiff` only accepts WhyBroken results in v1. It formats evidence already
on the result; it does not re-run diagnostics.

---

## Handling structured errors

```go
import (
	"fmt"

	"github.com/BaimPriyatna/repro/src/core/errors"
)

func handle(err error) {
	if errors.Is(err, errors.CodeNotFound) {
		fmt.Println("missing resource")
		return
	}
	if re, ok := err.(*errors.ReproError); ok {
		fmt.Printf("[%s] %s\n", re.Code, re.Message)
		return
	}
	fmt.Println(err)
}
```

---

## More detail

- Architecture and layer rules: [ARCHITECTURE.md](./ARCHITECTURE.md)
- Module ADRs: [`docs/adr/`](./docs/adr/)
- Build and test: [CONTRIBUTING.md](./CONTRIBUTING.md)
