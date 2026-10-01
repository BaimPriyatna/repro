package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/absent"
	"github.com/BaimPriyatna/repro/src/analysis/beforeafter"
	"github.com/BaimPriyatna/repro/src/analysis/changemap"
	"github.com/BaimPriyatna/repro/src/analysis/drift"
	"github.com/BaimPriyatna/repro/src/analysis/history"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

var diffCmd = &cobra.Command{
	Use:   "diff [from] [to]",
	Short: "Compute structural diff between two snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := flagFrom
		toID := flagTo
		if fromID == "" && len(args) > 0 {
			fromID = args[0]
		}
		if toID == "" && len(args) > 1 {
			toID = args[1]
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		snapA, snapB, err := resolveSnapshotPair(ctx, snapStore, fromID, toID)
		if err != nil {
			return err
		}

		analyzer, err := beforeafter.New(snapA, snapB)
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

var historyCmd = &cobra.Command{
	Use:   "history [snapshot]",
	Short: "Traverse snapshot lineage history",
	RunE: func(cmd *cobra.Command, args []string) error {
		snapID := flagSnapshot
		if snapID == "" && len(args) > 0 {
			snapID = args[0]
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if snapID == "" {
			snaps, err := snapStore.List(ctx, store.Filter{Limit: 1})
			if err != nil || len(snaps) == 0 {
				return errors.New(errors.CodeNotFound, "no snapshots found in store")
			}
			snapID = string(snaps[0].ID)
		}

		traverser := history.NewTraverser(&storeLoader{snapStore})
		maxDepth := flagMaxDepth
		if maxDepth <= 0 {
			maxDepth = 20
		}

		chain, err := traverser.Traverse(analysis.HistoryQuery{
			StartID:      snapshot.ID(snapID),
			IncludeStart: true,
			MaxDepth:     maxDepth,
		})
		if err != nil {
			return err
		}

		return renderOutput(cmd, chain, func() (string, error) {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Lineage for %s (depth: %d):\n", snapID, len(chain.Snapshots)))
			for i, s := range chain.Snapshots {
				b.WriteString(fmt.Sprintf(" %2d. %s  %s  %s\n",
					i+1,
					s.ID,
					s.Timestamp.Format("2006-01-02 15:04:05"),
					s.Source,
				))
			}
			return b.String(), nil
		})
	},
}

var driftCmd = &cobra.Command{
	Use:   "drift [from] [to]",
	Short: "Analyze gradual configuration drift from a baseline snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		baseID := flagFrom
		if baseID == "" && len(args) > 0 {
			baseID = args[0]
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if baseID == "" {
			snaps, err := snapStore.List(ctx, store.Filter{Limit: 1})
			if err != nil || len(snaps) == 0 {
				return errors.New(errors.CodeNotFound, "no snapshots found in store")
			}
			baseID = string(snaps[0].ID)
		}

		baseline, err := snapStore.Load(ctx, snapshot.ID(baseID))
		if err != nil {
			return err
		}

		traverser := history.NewTraverser(&storeLoader{snapStore})
		analyzer, err := drift.New(baseline, traverser)
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

var absentCmd = &cobra.Command{
	Use:   "absent [from] [to]",
	Short: "Detect entities present before but missing now",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := flagFrom
		toID := flagTo
		if fromID == "" && len(args) > 0 {
			fromID = args[0]
		}
		if toID == "" && len(args) > 1 {
			toID = args[1]
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		snapA, snapB, err := resolveSnapshotPair(ctx, snapStore, fromID, toID)
		if err != nil {
			return err
		}

		analyzer, err := absent.New(snapA, snapB)
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

var changeMapCmd = &cobra.Command{
	Use:   "change-map [snapshot]",
	Short: "Build temporal map of changes across snapshot history",
	RunE: func(cmd *cobra.Command, args []string) error {
		snapID := flagSnapshot
		if snapID == "" && len(args) > 0 {
			snapID = args[0]
		}

		snapStore, _, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		var startSnap *snapshot.Snapshot
		if snapID == "" {
			snaps, err := snapStore.List(ctx, store.Filter{Limit: 1})
			if err != nil || len(snaps) == 0 {
				return errors.New(errors.CodeNotFound, "no snapshots found in store")
			}
			startSnap = snaps[0]
		} else {
			startSnap, err = snapStore.Load(ctx, snapshot.ID(snapID))
			if err != nil {
				return err
			}
		}

		traverser := history.NewTraverser(&storeLoader{snapStore})
		analyzer, err := changemap.New(startSnap, traverser)
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
	diffCmd.Flags().StringVar(&flagFrom, "from", "", "Source snapshot ID")
	diffCmd.Flags().StringVar(&flagTo, "to", "", "Target snapshot ID")

	historyCmd.Flags().StringVar(&flagSnapshot, "snapshot", "", "Starting snapshot ID")
	historyCmd.Flags().IntVar(&flagMaxDepth, "max-depth", 20, "Maximum traversal depth")

	driftCmd.Flags().StringVar(&flagFrom, "from", "", "Baseline snapshot ID")

	absentCmd.Flags().StringVar(&flagFrom, "from", "", "Baseline snapshot ID")
	absentCmd.Flags().StringVar(&flagTo, "to", "", "Target snapshot ID")

	changeMapCmd.Flags().StringVar(&flagSnapshot, "snapshot", "", "Starting snapshot ID")
}
