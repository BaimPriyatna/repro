# Go Library Usage Examples

Practical code examples demonstrating how to use Repro as a Go library.

---

## 1. Capturing and Comparing Snapshots

Capture two snapshots programmatically and compare them:

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

	// In-memory store (use store.NewFileStore("/path/to/dir") for persistence)
	memStore := store.NewMemStore()
	eng, err := engine.New(memStore)
	if err != nil {
		log.Fatalf("engine initialization failed: %v", err)
	}

	// Capture initial baseline
	before, err := eng.Capture(ctx, snapshot.SourceManual)
	if err != nil {
		log.Fatalf("capture failed: %v", err)
	}
	if err := eng.Store(ctx, before); err != nil {
		log.Fatalf("store failed: %v", err)
	}

	// Capture second snapshot with parent reference
	after, err := eng.Capture(ctx, snapshot.SourceManual, engine.WithParent(before.ID))
	if err != nil {
		log.Fatalf("capture failed: %v", err)
	}
	if err := eng.Store(ctx, after); err != nil {
		log.Fatalf("store failed: %v", err)
	}

	// Compare states
	cmp, err := eng.Compare(before, after)
	if err != nil {
		log.Fatalf("comparison failed: %v", err)
	}

	fmt.Printf("States identical: %v, Modified entities: %d\n", cmp.Identical, len(cmp.ModifiedKeys))
}
```

---

## 2. Running BeforeAfter Analysis

Run automated difference analysis and extract evidence-backed findings:

```go
package main

import (
	"fmt"
	"log"

	"github.com/BaimPriyatna/repro/src/analysis/beforeafter"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func AnalyzeStates(before, after *snapshot.Snapshot) {
	ba, err := beforeafter.New(before, after)
	if err != nil {
		log.Fatal(err)
	}

	result, err := ba.Analyze()
	if err != nil {
		log.Fatal(err)
	}

	for _, finding := range result.Findings {
		fmt.Printf("[%s / %s] %s: %s\n", finding.Severity, finding.Confidence, finding.ID, finding.Type)
		for _, ev := range finding.Evidence {
			fmt.Printf("  Evidence [%s]: %s\n", ev.Kind, ev.SourceID)
		}
	}
}
```

---

## 3. Formatting Explanations (WhyBroken to HumanReadable)

Convert an analysis result into a human-readable diagnosis narrative:

```go
package main

import (
	"fmt"
	"log"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
	"github.com/BaimPriyatna/repro/src/presentation/humanreadable"
)

func RenderDiagnostic(result *coreanalysis.AnalysisResult) {
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
