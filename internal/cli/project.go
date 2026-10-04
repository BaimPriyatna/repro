package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/internal/config"
	"github.com/BaimPriyatna/repro/src/central"
	"github.com/BaimPriyatna/repro/src/core/errors"
)

func resolveCentralStore() (*central.CentralStore, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}
	dataDir := cfg.Storage.DataDir
	if flagDataDir != "" {
		dataDir = flagDataDir
	}
	csDir := filepath.Join(dataDir, "store")
	return central.NewCentralStore(csDir)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize and register the current directory in central storage",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(errors.CodeStorageFailure, "getting current directory", err)
		}

		meta, warning, err := cs.InitProject(cwd, flagName)
		if err != nil {
			return err
		}

		if warning != "" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), warning)
		}

		return renderOutput(cmd, meta, func() (string, error) {
			var b strings.Builder
			b.WriteString("Initialized Repro project:\n")
			b.WriteString(fmt.Sprintf("  ID:   %s\n", meta.ID))
			b.WriteString(fmt.Sprintf("  Name: %s\n", meta.Name))
			b.WriteString(fmt.Sprintf("  Path: %s\n", cwd))
			if meta.GitInitialCommit != "" {
				b.WriteString(fmt.Sprintf("  Git Initial Commit: %s\n", meta.GitInitialCommit))
			}
			return b.String(), nil
		})
	},
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a known directory path or record a development event",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If called without a subcommand and with a path argument, behave as path-add.
		if len(args) == 0 {
			return errors.New(errors.CodeInvalidInput, "usage: repro add <path>  or  repro add event --type ... --desc ...")
		}

		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		cwd, _ := os.Getwd()
		meta, err := cs.ResolveProject(cwd)
		if err != nil {
			return err
		}

		targetPath := args[0]
		if err := cs.AddPath(meta.ID, targetPath); err != nil {
			return err
		}

		return renderOutput(cmd, fmt.Sprintf("Added path %q to project %s\n", targetPath, meta.Name), nil)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		projects, err := cs.ListProjects()
		if err != nil {
			return err
		}

		return renderOutput(cmd, projects, func() (string, error) {
			if len(projects) == 0 {
				return "No registered projects.\n", nil
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%-38s %-20s %-10s %s\n", "ID", "NAME", "ARCHIVED", "KNOWN PATH"))
			b.WriteString(strings.Repeat("-", 80))
			b.WriteString("\n")
			for _, p := range projects {
				path := ""
				if len(p.KnownPaths) > 0 {
					path = p.KnownPaths[0]
				}
				b.WriteString(fmt.Sprintf("%-38s %-20s %-10v %s\n", p.ID, p.Name, p.Archived, path))
			}
			return b.String(), nil
		})
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status and resolution details of the current project",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		cwd, _ := os.Getwd()
		meta, err := cs.ResolveProject(cwd)
		if err != nil {
			return err
		}

		match, host := cs.CheckHostname(meta)
		if !match && !flagYes {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Notice: project was last accessed on host %q (current host: %q)\n", meta.LastHostname, host)
		}

		stats, err := cs.ProjectStats(meta.ID)
		if err != nil {
			return err
		}

		return renderOutput(cmd, stats, func() (string, error) {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Project:        %s (%s)\n", meta.Name, meta.ID))
			b.WriteString(fmt.Sprintf("Status:         %s\n", stats.Status))
			b.WriteString(fmt.Sprintf("Snapshots:      %d\n", stats.SnapshotCount))
			b.WriteString(fmt.Sprintf("Events:         %d\n", stats.EventCount))
			b.WriteString(fmt.Sprintf("Last Activity:  %s (%d days ago)\n", stats.LastActivity.Format("2006-01-02 15:04:05"), stats.DaysSinceActive))
			b.WriteString(fmt.Sprintf("Host:           %s\n", host))
			return b.String(), nil
		})
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats [project-id]",
	Short: "Display statistics for a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		var projID central.ProjectID
		if len(args) > 0 {
			projID = central.ProjectID(args[0])
		} else {
			cwd, _ := os.Getwd()
			meta, err := cs.ResolveProject(cwd)
			if err != nil {
				return err
			}
			projID = meta.ID
		}

		stats, err := cs.ProjectStats(projID)
		if err != nil {
			return err
		}

		return renderOutput(cmd, stats, nil)
	},
}

var cemeteryCmd = &cobra.Command{
	Use:   "cemetery",
	Short: "List project cemetery and inactivity metrics",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		entries, err := cs.ListCemetery()
		if err != nil {
			return err
		}

		return renderOutput(cmd, entries, func() (string, error) {
			if len(entries) == 0 {
				return "Cemetery is empty.\n", nil
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%-20s %-12s %-12s %-8s %s\n", "NAME", "STATUS", "INACTIVE", "SNAPS", "PATH"))
			b.WriteString(strings.Repeat("-", 80))
			b.WriteString("\n")
			for _, e := range entries {
				b.WriteString(fmt.Sprintf("%-20s %-12s %-12s %-8d %s\n",
					e.Name,
					e.Status,
					fmt.Sprintf("%d days", e.DaysSinceActive),
					e.SnapshotCount,
					e.LastPath,
				))
			}
			return b.String(), nil
		})
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive <project-id>",
	Short: "Mark a project as archived",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}
		projID := central.ProjectID(args[0])
		if err := cs.Archive(projID); err != nil {
			return err
		}
		return renderOutput(cmd, fmt.Sprintf("Archived project %s\n", projID), nil)
	},
}

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive <project-id>",
	Short: "Restore an archived project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}
		projID := central.ProjectID(args[0])
		if err := cs.Unarchive(projID); err != nil {
			return err
		}
		return renderOutput(cmd, fmt.Sprintf("Unarchived project %s\n", projID), nil)
	},
}

var forgetCmd = &cobra.Command{
	Use:   "forget <project-id>",
	Short: "Safely remove a project from index without deleting data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}
		projID := central.ProjectID(args[0])
		if err := cs.Forget(projID); err != nil {
			return err
		}
		return renderOutput(cmd, fmt.Sprintf("Project %s removed from central index (data retained on disk).\n", projID), nil)
	},
}

var setThresholdCmd = &cobra.Command{
	Use:   "set-threshold <project-id>",
	Short: "Configure inactivity day thresholds for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}
		projID := central.ProjectID(args[0])
		meta, err := cs.GetProject(projID)
		if err != nil {
			return err
		}

		thresholds := meta.Thresholds
		if flagActiveDays > 0 {
			thresholds.ActiveDays = flagActiveDays
		}
		if flagIdleDays > 0 {
			thresholds.IdleDays = flagIdleDays
		}
		if flagDormantDays > 0 {
			thresholds.DormantDays = flagDormantDays
		}

		if err := cs.SetThresholds(projID, thresholds); err != nil {
			return err
		}

		return renderOutput(cmd, fmt.Sprintf("Updated thresholds for project %s: active=%d, idle=%d, dormant=%d\n",
			projID, thresholds.ActiveDays, thresholds.IdleDays, thresholds.DormantDays), nil)
	},
}

var migrateToCentralCmd = &cobra.Command{
	Use:   "migrate-to-central",
	Short: "Copy local snapshots and events to the central store non-destructively",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cs, err := resolveCentralStore()
		if err != nil {
			return err
		}

		fromDir := flagFromDir
		if fromDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			fromDir = cwd
		}

		meta, err := cs.MigrateToCentral(fromDir, flagName)
		if err != nil {
			return err
		}

		return renderOutput(cmd, fmt.Sprintf("Successfully migrated %q into central store as project %s (%s)\n",
			fromDir, meta.Name, meta.ID), nil)
	},
}

func init() {
	initCmd.Flags().StringVar(&flagName, "name", "", "Custom name for the project")

	statusCmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Confirm cross-machine usage without warning")

	setThresholdCmd.Flags().IntVar(&flagActiveDays, "active-days", 7, "Days before idle")
	setThresholdCmd.Flags().IntVar(&flagIdleDays, "idle-days", 30, "Days before dormant")
	setThresholdCmd.Flags().IntVar(&flagDormantDays, "dormant-days", 90, "Days before abandoned")

	migrateToCentralCmd.Flags().StringVar(&flagFromDir, "from-dir", "", "Source directory containing local .repro")
	migrateToCentralCmd.Flags().StringVar(&flagName, "name", "", "Custom project name in central store")

	// Register add subcommands.
	addCmd.AddCommand(addEventCmd)
}
