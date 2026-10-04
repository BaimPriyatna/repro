package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// validEventTypes maps user-facing type strings to canonical event.Type constants.
// Accepts both the exact constant name and common aliases.
var validEventTypes = map[string]event.Type{
	"FILE_CHANGED":       event.TypeFileChanged,
	"file_changed":       event.TypeFileChanged,
	"PACKAGE_INSTALLED":  event.TypePackageInstalled,
	"package_installed":  event.TypePackageInstalled,
	"dependency_install": event.TypePackageInstalled,
	"PACKAGE_REMOVED":    event.TypePackageRemoved,
	"package_removed":    event.TypePackageRemoved,
	"dependency_remove":  event.TypePackageRemoved,
	"CONFIG_CHANGED":     event.TypeConfigChanged,
	"config_changed":     event.TypeConfigChanged,
	"GIT_COMMIT":         event.TypeGitCommit,
	"git_commit":         event.TypeGitCommit,
	"COMMAND_EXECUTED":   event.TypeCommandExecuted,
	"command_executed":   event.TypeCommandExecuted,
	"RUNTIME_CHANGED":    event.TypeRuntimeChanged,
	"runtime_changed":    event.TypeRuntimeChanged,
	"SNAPSHOT_CREATED":   event.TypeSnapshotCreated,
	"snapshot_created":   event.TypeSnapshotCreated,
	"SNAPSHOT_DELETED":   event.TypeSnapshotDeleted,
	"snapshot_deleted":   event.TypeSnapshotDeleted,
	// docs alias used in quickstart
	"dependency_update": event.TypePackageInstalled,
}

var (
	flagEventType    string
	flagEventSource  string
	flagEventDesc    string
	flagEventSnap    string
)

var addEventCmd = &cobra.Command{
	Use:   "event",
	Short: "Record a development event",
	Long: `Record a development event such as a dependency update, config change, or command execution.

Valid event types:
  FILE_CHANGED, PACKAGE_INSTALLED, PACKAGE_REMOVED, CONFIG_CHANGED,
  GIT_COMMIT, COMMAND_EXECUTED, RUNTIME_CHANGED, SNAPSHOT_CREATED,
  SNAPSHOT_DELETED

  Aliases accepted: dependency_update, dependency_install, dependency_remove,
  file_changed, config_changed, git_commit, command_executed, runtime_changed.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagEventType == "" {
			return errors.New(errors.CodeInvalidInput, "--type is required (e.g. dependency_update, config_changed, command_executed)")
		}
		if flagEventDesc == "" {
			return errors.New(errors.CodeInvalidInput, "--desc is required: a short description of the event subject")
		}

		evType, ok := validEventTypes[flagEventType]
		if !ok {
			// Build sorted list for the error message.
			known := []string{
				"FILE_CHANGED", "PACKAGE_INSTALLED", "PACKAGE_REMOVED",
				"CONFIG_CHANGED", "GIT_COMMIT", "COMMAND_EXECUTED",
				"RUNTIME_CHANGED", "SNAPSHOT_CREATED", "SNAPSHOT_DELETED",
				"dependency_update", "dependency_install", "dependency_remove",
			}
			return errors.New(errors.CodeInvalidInput,
				fmt.Sprintf("unknown event type %q — valid types: %s", flagEventType, strings.Join(known, ", ")))
		}

		src := event.Source(flagEventSource)
		if src == "" {
			src = event.SourceUser
		}

		opts := []event.Option{}
		if flagEventSnap != "" {
			opts = append(opts, event.WithSnapshot(snapshot.ID(flagEventSnap)))
		}

		ev, err := event.New(evType, src, flagEventDesc, opts...)
		if err != nil {
			return err
		}

		_, evStore, err := resolveStores()
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := evStore.Record(ctx, ev); err != nil {
			return errors.Wrap(errors.CodeStorageFailure, "saving event", err)
		}

		return renderOutput(cmd, ev, func() (string, error) {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Event recorded:\n"))
			b.WriteString(fmt.Sprintf("  ID:        %s\n", ev.ID))
			b.WriteString(fmt.Sprintf("  Type:      %s\n", ev.Type))
			b.WriteString(fmt.Sprintf("  Source:    %s\n", ev.Source))
			b.WriteString(fmt.Sprintf("  Subject:   %s\n", ev.Subject))
			b.WriteString(fmt.Sprintf("  Timestamp: %s\n", ev.Timestamp.Format("2006-01-02 15:04:05 UTC")))
			if ev.RelatedSnapshotID != "" {
				b.WriteString(fmt.Sprintf("  Snapshot:  %s\n", ev.RelatedSnapshotID))
			}
			return b.String(), nil
		})
	},
}

func init() {
	addEventCmd.Flags().StringVar(&flagEventType, "type", "", "Event type (e.g. dependency_update, config_changed, command_executed)")
	addEventCmd.Flags().StringVar(&flagEventSource, "source", "user", "Event source (default: user)")
	addEventCmd.Flags().StringVar(&flagEventDesc, "desc", "", "Short description / subject of the event")
	addEventCmd.Flags().StringVar(&flagEventSnap, "snapshot", "", "Associate event with a snapshot ID (optional)")
	_ = addEventCmd.MarkFlagRequired("type")
	_ = addEventCmd.MarkFlagRequired("desc")
}
