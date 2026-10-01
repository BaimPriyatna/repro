package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	"github.com/BaimPriyatna/repro/src/analysis/whybroken"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/engine/store"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	graphstore "github.com/BaimPriyatna/repro/src/graph/store"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
	"github.com/BaimPriyatna/repro/src/presentation/humanreadable"
)

var explainDiffCmd = &cobra.Command{
	Use:   "explain-diff",
	Short: "Explain root-cause analysis findings citing diagnostic evidence",
	RunE: func(cmd *cobra.Command, _ []string) error {
		analysisRes, diffRes, err := resolveAnalysisAndDiff()
		if err != nil {
			return err
		}

		opts := []explaindiff.Option{}
		if diffRes != nil {
			opts = append(opts, explaindiff.WithDiff(diffRes))
		}
		if flagContext != "" {
			opts = append(opts, explaindiff.WithContext(flagContext))
		}

		explainer, err := explaindiff.New(analysisRes, opts...)
		if err != nil {
			return err
		}

		explanation, err := explainer.Explain()
		if err != nil {
			return err
		}

		return renderOutput(cmd, explanation, func() (string, error) {
			renderer, err := humanreadable.New(explanation)
			if err != nil {
				return "", err
			}
			diag, err := renderer.Render()
			if err != nil {
				return "", err
			}
			return diag.Text, nil
		})
	},
}

var humanReadableCmd = &cobra.Command{
	Use:   "human-readable",
	Short: "Render diagnostic results as plain language without altering diagnosis",
	RunE: func(cmd *cobra.Command, _ []string) error {
		var explanation *explaindiff.Explanation

		if flagResult != "" {
			data, err := os.ReadFile(flagResult)
			if err != nil {
				return errors.Wrap(errors.CodeInvalidInput, "reading result file", err)
			}

			var exp explaindiff.Explanation
			if err := json.Unmarshal(data, &exp); err == nil && exp.ID != "" && exp.SourceResultID != "" {
				explanation = &exp
			} else {
				var ar coreanalysis.AnalysisResult
				if err := json.Unmarshal(data, &ar); err != nil {
					return errors.Wrap(errors.CodeInvalidInput, "unmarshaling result JSON (expected Explanation or AnalysisResult)", err)
				}
				explainer, err := explaindiff.New(&ar, explaindiff.WithContext(flagContext))
				if err != nil {
					return err
				}
				explanation, err = explainer.Explain()
				if err != nil {
					return err
				}
			}
		} else {
			analysisRes, diffRes, err := resolveAnalysisAndDiff()
			if err != nil {
				return err
			}
			opts := []explaindiff.Option{}
			if diffRes != nil {
				opts = append(opts, explaindiff.WithDiff(diffRes))
			}
			if flagContext != "" {
				opts = append(opts, explaindiff.WithContext(flagContext))
			}
			explainer, err := explaindiff.New(analysisRes, opts...)
			if err != nil {
				return err
			}
			explanation, err = explainer.Explain()
			if err != nil {
				return err
			}
		}

		renderer, err := humanreadable.New(explanation)
		if err != nil {
			return err
		}

		diagnosis, err := renderer.Render()
		if err != nil {
			return err
		}

		return renderOutput(cmd, diagnosis, func() (string, error) {
			return diagnosis.Text, nil
		})
	},
}

func resolveAnalysisAndDiff() (*coreanalysis.AnalysisResult, *analysis.DiffResult, error) {
	if flagResult != "" {
		data, err := os.ReadFile(flagResult)
		if err != nil {
			return nil, nil, errors.Wrap(errors.CodeInvalidInput, "reading result file", err)
		}
		var ar coreanalysis.AnalysisResult
		if err := json.Unmarshal(data, &ar); err != nil {
			return nil, nil, errors.Wrap(errors.CodeInvalidInput, "unmarshaling AnalysisResult JSON", err)
		}
		return &ar, nil, nil
	}

	snapStore, evStore, err := resolveStores()
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()

	fromID := flagFrom
	toID := flagTo
	if fromID == "" || toID == "" {
		snaps, err := snapStore.List(ctx, store.Filter{Limit: 2})
		if err != nil || len(snaps) < 2 {
			return nil, nil, errors.New(errors.CodeInvalidInput, "at least two snapshots required; specify --from and --to")
		}
		toID = string(snaps[0].ID)
		fromID = string(snaps[1].ID)
	}

	snapA, snapB, err := resolveSnapshotPair(ctx, snapStore, fromID, toID)
	if err != nil {
		return nil, nil, err
	}

	var rm *repairmap.RepairMap
	if flagGraph != "" {
		rm, err = loadGraphFromFile(flagGraph)
		if err != nil {
			return nil, nil, err
		}
	} else {
		rm, _ = repairmap.New(graphstore.NewMemStore())
	}

	events, err := evStore.Query(ctx, event.Query{
		After:  snapA.Timestamp,
		Before: snapB.Timestamp,
	})
	if err != nil {
		events = []*event.Event{}
	}

	opts := []whybroken.Option{}
	if flagSubject != "" {
		opts = append(opts, whybroken.WithSubject(entity.ID(flagSubject)))
	}

	analyzer, err := whybroken.New(rm, snapA, snapB, events, opts...)
	if err != nil {
		return nil, nil, err
	}

	res, err := analyzer.Analyze()
	if err != nil {
		return nil, nil, err
	}

	diffRes, _ := diff.Diff(snapA, snapB)
	return res, diffRes, nil
}

func init() {
	explainDiffCmd.Flags().StringVar(&flagResult, "result", "", "Path to AnalysisResult JSON file")
	explainDiffCmd.Flags().StringVar(&flagFrom, "from", "", "Baseline snapshot ID")
	explainDiffCmd.Flags().StringVar(&flagTo, "to", "", "Broken snapshot ID")
	explainDiffCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	explainDiffCmd.Flags().StringVar(&flagSubject, "subject", "", "Failing subject entity ID")
	explainDiffCmd.Flags().StringVar(&flagContext, "context", "", "Situational context")

	humanReadableCmd.Flags().StringVar(&flagResult, "result", "", "Path to Explanation or AnalysisResult JSON file")
	humanReadableCmd.Flags().StringVar(&flagFrom, "from", "", "Baseline snapshot ID")
	humanReadableCmd.Flags().StringVar(&flagTo, "to", "", "Broken snapshot ID")
	humanReadableCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	humanReadableCmd.Flags().StringVar(&flagSubject, "subject", "", "Failing subject entity ID")
	humanReadableCmd.Flags().StringVar(&flagContext, "context", "", "Situational context")
}
