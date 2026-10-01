package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/deadconfig"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	"github.com/BaimPriyatna/repro/src/analysis/ghostfile"
	"github.com/BaimPriyatna/repro/src/analysis/history"
	"github.com/BaimPriyatna/repro/src/analysis/impact"
	"github.com/BaimPriyatna/repro/src/analysis/orphan"
	"github.com/BaimPriyatna/repro/src/analysis/whybroken"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/store"
	"github.com/BaimPriyatna/repro/src/graph/depspy"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	graphstore "github.com/BaimPriyatna/repro/src/graph/store"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
	"github.com/BaimPriyatna/repro/src/presentation/humanreadable"
)

var whyBrokenCmd = &cobra.Command{
	Use:   "why-broken",
	Short: "Perform evidence-backed root cause analysis",
	RunE: func(cmd *cobra.Command, _ []string) error {
		snapStore, evStore, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()

		fromID := flagFrom
		toID := flagTo
		if fromID == "" || toID == "" {
			snaps, err := snapStore.List(ctx, store.Filter{Limit: 2})
			if err != nil || len(snaps) < 2 {
				return errors.New(errors.CodeInvalidInput, "at least two snapshots required; specify --from and --to")
			}
			toID = string(snaps[0].ID)
			fromID = string(snaps[1].ID)
		}

		snapA, snapB, err := resolveSnapshotPair(ctx, snapStore, fromID, toID)
		if err != nil {
			return err
		}

		var rm *repairmap.RepairMap
		if flagGraph != "" {
			rm, err = loadGraphFromFile(flagGraph)
			if err != nil {
				return err
			}
		} else {
			rm, _ = repairmap.New(graphstore.NewMemStore())
		}

		// Pre-filter events between snapshots
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
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, func() (string, error) {
			diffRes, err := diff.Diff(snapA, snapB)
			if err != nil {
				return defaultHumanFormat(res), nil
			}
			exp, err := explaindiff.New(res, explaindiff.WithDiff(diffRes))
			if err != nil {
				return defaultHumanFormat(res), nil
			}
			explanation, err := exp.Explain()
			if err != nil {
				return defaultHumanFormat(res), nil
			}
			renderer, err := humanreadable.New(explanation)
			if err != nil {
				return defaultHumanFormat(res), nil
			}
			diag, err := renderer.Render()
			if err != nil {
				return defaultHumanFormat(res), nil
			}
			return diag.Text, nil
		})
	},
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Detect hidden and undeclared dependencies",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagDeclared == "" || flagObserved == "" {
			return errors.New(errors.CodeInvalidInput, "both --declared and --observed graph JSON paths are required")
		}

		rmDecl, err := loadGraphFromFile(flagDeclared)
		if err != nil {
			return err
		}
		rmObs, err := loadGraphFromFile(flagObserved)
		if err != nil {
			return err
		}

		analyzer, err := depspy.New(rmDecl.Store(), rmObs.Store())
		if err != nil {
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, nil)
	},
}

var repairMapCmd = &cobra.Command{
	Use:   "repair-map",
	Short: "Query dependency relationships in the repair map",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagGraph == "" {
			return errors.New(errors.CodeInvalidInput, "--graph JSON file path is required")
		}

		rm, err := loadGraphFromFile(flagGraph)
		if err != nil {
			return err
		}

		if flagSubject != "" {
			deps, err := rm.DirectDependencies(entity.ID(flagSubject), "")
			if err != nil {
				return err
			}
			dependents, err := rm.DirectDependents(entity.ID(flagSubject), "")
			if err != nil {
				return err
			}
			data := map[string]any{
				"subject":      flagSubject,
				"dependencies": deps,
				"dependents":   dependents,
			}
			return renderOutput(cmd, data, func() (string, error) {
				var b strings.Builder
				b.WriteString(fmt.Sprintf("Entity: %s\n", flagSubject))
				b.WriteString(fmt.Sprintf("Direct Dependencies (%d):\n", len(deps)))
				for _, d := range deps {
					b.WriteString(fmt.Sprintf(" - %s (%s)\n", d.ID, d.Kind))
				}
				b.WriteString(fmt.Sprintf("Direct Dependents (%d):\n", len(dependents)))
				for _, d := range dependents {
					b.WriteString(fmt.Sprintf(" - %s (%s)\n", d.ID, d.Kind))
				}
				return b.String(), nil
			})
		}

		data := map[string]any{
			"entity_count":   rm.EntityCount(),
			"relation_count": rm.RelationCount(),
		}
		return renderOutput(cmd, data, func() (string, error) {
			return fmt.Sprintf("RepairMap: %d entities, %d relations\n", rm.EntityCount(), rm.RelationCount()), nil
		})
	},
}

var impactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Analyze blast radius and potentially affected components",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagGraph == "" {
			return errors.New(errors.CodeInvalidInput, "--graph JSON file path is required")
		}
		if len(flagEntities) == 0 {
			return errors.New(errors.CodeInvalidInput, "at least one --entity ID is required")
		}

		rm, err := loadGraphFromFile(flagGraph)
		if err != nil {
			return err
		}

		entityIDs := make([]entity.ID, len(flagEntities))
		for i, id := range flagEntities {
			entityIDs[i] = entity.ID(id)
		}

		maxDepth := flagMaxDepth
		if maxDepth <= 0 {
			maxDepth = 50
		}

		analyzer, err := impact.New(rm, entityIDs, impact.WithMaxDepth(maxDepth))
		if err != nil {
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, nil)
	},
}

var deadConfigCmd = &cobra.Command{
	Use:   "dead-config",
	Short: "Detect unused configuration items",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagGraph == "" {
			return errors.New(errors.CodeInvalidInput, "--graph JSON file path is required")
		}

		rm, err := loadGraphFromFile(flagGraph)
		if err != nil {
			return err
		}

		analyzer, err := deadconfig.New(rm)
		if err != nil {
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, nil)
	},
}

var orphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "Detect unowned and unreferenced resources",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagGraph == "" {
			return errors.New(errors.CodeInvalidInput, "--graph JSON file path is required")
		}

		rm, err := loadGraphFromFile(flagGraph)
		if err != nil {
			return err
		}

		analyzer, err := orphan.New(rm)
		if err != nil {
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, nil)
	},
}

var ghostFileCmd = &cobra.Command{
	Use:   "ghost-file",
	Short: "Analyze file lifecycle and identify abandoned files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagGraph == "" {
			return errors.New(errors.CodeInvalidInput, "--graph JSON file path is required")
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		var startSnap *snapshot.Snapshot
		if flagSnapshot != "" {
			startSnap, err = snapStore.Load(ctx, snapshot.ID(flagSnapshot))
			if err != nil {
				return err
			}
		} else {
			snaps, err := snapStore.List(ctx, store.Filter{Limit: 1})
			if err != nil || len(snaps) == 0 {
				return errors.New(errors.CodeNotFound, "no snapshots found in store")
			}
			startSnap = snaps[0]
		}

		traverser := history.NewTraverser(&storeLoader{snapStore})
		chain, err := traverser.Traverse(analysis.HistoryQuery{
			StartID:      startSnap.ID,
			IncludeStart: true,
			MaxDepth:     20,
		})
		if err != nil {
			return err
		}

		rm, err := loadGraphFromFile(flagGraph)
		if err != nil {
			return err
		}

		analyzer, err := ghostfile.New(rm, chain)
		if err != nil {
			return err
		}

		res, err := analyzer.Analyze()
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, nil)
	},
}

func init() {
	whyBrokenCmd.Flags().StringVar(&flagFrom, "from", "", "Baseline snapshot ID")
	whyBrokenCmd.Flags().StringVar(&flagTo, "to", "", "Broken snapshot ID")
	whyBrokenCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	whyBrokenCmd.Flags().StringVar(&flagSubject, "subject", "", "Failing subject entity ID")

	depsCmd.Flags().StringVar(&flagDeclared, "declared", "", "Path to declared graph JSON")
	depsCmd.Flags().StringVar(&flagObserved, "observed", "", "Path to observed graph JSON")

	repairMapCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	repairMapCmd.Flags().StringVar(&flagSubject, "subject", "", "Entity ID to inspect")

	impactCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	impactCmd.Flags().StringSliceVar(&flagEntities, "entity", nil, "Changed entity IDs (repeatable)")
	impactCmd.Flags().IntVar(&flagMaxDepth, "max-depth", 50, "Max traversal depth")

	deadConfigCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")

	orphanCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")

	ghostFileCmd.Flags().StringVar(&flagGraph, "graph", "", "Path to graph JSON file")
	ghostFileCmd.Flags().StringVar(&flagSnapshot, "snapshot", "", "Snapshot ID to evaluate")
}
