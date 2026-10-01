package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/intelligence/manualtrace"
)

var manualTraceCmd = &cobra.Command{
	Use:   "manual-trace",
	Short: "Detect repetitive manual command sequences and suggest automation",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, evStore, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		query := event.Query{Order: event.SortAsc}

		if flagAfter != "" {
			t, err := time.Parse(time.RFC3339, flagAfter)
			if err != nil {
				return errors.Wrap(errors.CodeInvalidInput, "parsing --after timestamp", err)
			}
			query.After = t
		}
		if flagBefore != "" {
			t, err := time.Parse(time.RFC3339, flagBefore)
			if err != nil {
				return errors.Wrap(errors.CodeInvalidInput, "parsing --before timestamp", err)
			}
			query.Before = t
		}

		analyzer := manualtrace.New()
		res, err := analyzer.AnalyzeStore(ctx, evStore, query)
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, func() (string, error) {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Scanned %d events across %d session(s).\n", res.EventsScanned, res.Sessions))
			if len(res.Patterns) == 0 {
				b.WriteString("No repetitive workflow patterns detected.\n")
				return b.String(), nil
			}

			b.WriteString(fmt.Sprintf("\nDetected %d repetitive pattern(s):\n", len(res.Patterns)))
			for i, p := range res.Patterns {
				b.WriteString(fmt.Sprintf("\nPattern #%d (Occurrences: %d):\n", i+1, p.Occurrences))
				for _, step := range p.Steps {
					b.WriteString(fmt.Sprintf("  - %s\n", step))
				}
				b.WriteString(fmt.Sprintf("  Suggestion: %s\n", p.Suggestion))
			}
			return b.String(), nil
		})
	},
}

func init() {
	manualTraceCmd.Flags().StringVar(&flagAfter, "after", "", "Filter events after timestamp (RFC3339)")
	manualTraceCmd.Flags().StringVar(&flagBefore, "before", "", "Filter events before timestamp (RFC3339)")
}
