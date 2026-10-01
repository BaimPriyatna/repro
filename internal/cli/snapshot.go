package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [action]",
	Short: "Manage and inspect snapshots (capture, list, get, compare)",
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := resolveSnapshotEngine()
		if err != nil {
			return err
		}

		ctx := context.Background()

		action := flagAction
		if action == "" && len(args) > 0 {
			action = args[0]
		}
		if action == "" {
			action = "list"
		}

		switch strings.ToLower(action) {
		case "capture":
			labels := parseLabels(flagLabels)
			snap, err := eng.Capture(ctx, snapshot.SourceManual, engine.WithParent(snapshot.ID(flagParent)), engine.WithLabels(labels))
			if err != nil {
				return err
			}
			if err := eng.Store(ctx, snap); err != nil {
				return err
			}
			return renderOutput(cmd, snap, nil)

		case "list":
			snaps, err := eng.List(ctx, store.Filter{Limit: flagMaxDepth})
			if err != nil {
				return err
			}
			return renderOutput(cmd, snaps, func() (string, error) {
				if len(snaps) == 0 {
					return "No snapshots found.\n", nil
				}
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%-38s %-22s %-12s %-10s\n", "ID", "TIMESTAMP", "SOURCE", "HASH"))
				b.WriteString(strings.Repeat("-", 84))
				b.WriteString("\n")
				for _, s := range snaps {
					hashPrefix := s.ContentHash
					if len(hashPrefix) > 8 {
						hashPrefix = hashPrefix[:8]
					}
					b.WriteString(fmt.Sprintf("%-38s %-22s %-12s %-10s\n",
						s.ID,
						s.Timestamp.Format("2006-01-02 15:04:05"),
						s.Source,
						hashPrefix,
					))
				}
				return b.String(), nil
			})

		case "get":
			id := flagSnapshot
			if id == "" && len(args) > 1 {
				id = args[1]
			}
			if id == "" {
				return errors.New(errors.CodeInvalidInput, "snapshot ID is required (use --snapshot or positional argument)")
			}
			snap, err := eng.Load(ctx, snapshot.ID(id))
			if err != nil {
				return err
			}
			return renderOutput(cmd, snap, nil)

		case "compare":
			fromID := flagFrom
			toID := flagTo
			if fromID == "" && len(args) > 1 {
				fromID = args[1]
			}
			if toID == "" && len(args) > 2 {
				toID = args[2]
			}
			if fromID == "" || toID == "" {
				return errors.New(errors.CodeInvalidInput, "both from and to snapshot IDs required (use --from/--to or positional arguments)")
			}
			snapA, err := eng.Load(ctx, snapshot.ID(fromID))
			if err != nil {
				return err
			}
			snapB, err := eng.Load(ctx, snapshot.ID(toID))
			if err != nil {
				return err
			}
			cmp, err := eng.Compare(snapA, snapB)
			if err != nil {
				return err
			}
			return renderOutput(cmd, cmp, func() (string, error) {
				var b strings.Builder
				b.WriteString(fmt.Sprintf("Comparing %s -> %s\n", fromID, toID))
				b.WriteString(fmt.Sprintf("Identical: %v\n", cmp.Identical))
				allChanged := append(append(cmp.AddedKeys, cmp.RemovedKeys...), cmp.ModifiedKeys...)
				b.WriteString(fmt.Sprintf("Differences (%d):\n", len(allChanged)))
				for _, key := range allChanged {
					b.WriteString(fmt.Sprintf(" - %s\n", key))
				}
				return b.String(), nil
			})

		default:
			return errors.New(errors.CodeInvalidInput, fmt.Sprintf("unknown snapshot action: %q", action))
		}
	},
}

func init() {
	snapshotCmd.Flags().StringVar(&flagAction, "action", "", "Snapshot action (capture, list, get, compare)")
	snapshotCmd.Flags().StringVar(&flagSnapshot, "snapshot", "", "Snapshot ID")
	snapshotCmd.Flags().StringVar(&flagFrom, "from", "", "Source snapshot ID")
	snapshotCmd.Flags().StringVar(&flagTo, "to", "", "Target snapshot ID")
	snapshotCmd.Flags().StringVar(&flagParent, "parent", "", "Parent snapshot ID")
	snapshotCmd.Flags().IntVar(&flagMaxDepth, "max-depth", 0, "Max snapshots to list")
	snapshotCmd.Flags().StringSliceVar(&flagLabels, "label", nil, "Snapshot labels (k=v, repeatable)")
}
