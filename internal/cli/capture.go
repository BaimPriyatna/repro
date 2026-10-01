package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/capture"
	reprocap "github.com/BaimPriyatna/repro/src/capture/repro"
	"github.com/BaimPriyatna/repro/src/capture/timecapsule"
	"github.com/BaimPriyatna/repro/src/capture/watchdog"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture development environment state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		snapStore, evStore, err := resolveStores()
		if err != nil {
			return err
		}

		eng, err := resolveSnapshotEngine()
		if err != nil {
			return err
		}

		evSys, err := event.NewSystem(evStore)
		if err != nil {
			return err
		}

		labels := parseLabels(flagLabels)
		capCfg := &capture.Config{
			Engine:      eng,
			EventSystem: evSys,
			Labels:      labels,
		}

		ctx := context.Background()

		switch flagMode {
		case "", "oneshot":
			mod, err := reprocap.New(capCfg)
			if err != nil {
				return err
			}
			opts := []reprocap.CaptureOption{
				reprocap.WithLabels(labels),
				reprocap.WithReason(flagReason),
			}
			if flagParent != "" {
				opts = append(opts, reprocap.WithParent(snapshot.ID(flagParent)))
			}
			if flagEmitEvent {
				opts = append(opts, reprocap.WithEvent())
			}
			if len(flagCollectors) > 0 {
				opts = append(opts, reprocap.WithCollectors(flagCollectors...))
			}

			res, err := mod.CaptureAndStore(ctx, opts...)
			if err != nil {
				return err
			}
			return renderOutput(cmd, res.Snapshot, nil)

		case "periodic":
			mod, err := timecapsule.New(&timecapsule.Config{
				Capture:  capCfg,
				Interval: 10 * time.Minute,
			})
			if err != nil {
				return err
			}
			opts := []timecapsule.CaptureOption{
				timecapsule.WithLabels(labels),
				timecapsule.WithReason(flagReason),
			}
			if flagParent != "" {
				opts = append(opts, timecapsule.WithParent(snapshot.ID(flagParent)))
			}
			res, err := mod.Capture(ctx, opts...)
			if err != nil {
				return err
			}
			return renderOutput(cmd, res.Snapshot, nil)

		case "watch":
			mod, err := watchdog.New(&watchdog.Config{
				Capture:      capCfg,
				AutoSnapshot: true,
			})
			if err != nil {
				return err
			}
			res, err := mod.ReportChange(ctx, &watchdog.Change{
				Type:      watchdog.ChangeTypeConfigChanged,
				Subject:   "cli_trigger",
				Timestamp: time.Now().UTC(),
			})
			if err != nil {
				return err
			}
			if res.Snapshot != nil {
				return renderOutput(cmd, res.Snapshot, nil)
			}
			return renderOutput(cmd, fmt.Sprintf("Reported change with %d events emitted.", len(res.Events)), nil)

		default:
			_ = snapStore
			return errors.New(errors.CodeInvalidInput, fmt.Sprintf("unknown capture mode: %q (expected oneshot, periodic, or watch)", flagMode))
		}
	},
}

func init() {
	captureCmd.Flags().StringVar(&flagMode, "mode", "oneshot", "Capture mode: oneshot, periodic, or watch")
	captureCmd.Flags().StringVar(&flagReason, "reason", "", "Reason for capture")
	captureCmd.Flags().StringVar(&flagParent, "parent", "", "Parent snapshot ID")
	captureCmd.Flags().StringSliceVar(&flagCollectors, "collector", nil, "Restrict collectors (repeatable)")
	captureCmd.Flags().StringSliceVar(&flagLabels, "label", nil, "Snapshot labels (k=v, repeatable)")
	captureCmd.Flags().BoolVar(&flagEmitEvent, "emit-event", false, "Emit event after capture")
}
