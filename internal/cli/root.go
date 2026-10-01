// Package cli wires up the root Cobra command and all sub-commands.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRootCmd builds a fresh root command with all sub-commands attached.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "repro",
		Short:         "Local-first observability and diagnostic system for development environments",
		Long:          `Repro makes the state of a development environment observable, reproducible,` + "\n" + `comparable, explainable, and traceable over time.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flagFormat, "format", "human", "Output format: human or json")
	root.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Write output to file")
	root.PersistentFlags().StringVar(&flagDataDir, "data-dir", "", "Override storage data directory")
	root.PersistentFlags().BoolVar(&flagLocal, "local", false, "Operate in local standalone mode")

	root.AddCommand(
		captureCmd,
		snapshotCmd,
		diffCmd,
		historyCmd,
		driftCmd,
		absentCmd,
		changeMapCmd,
		whyBrokenCmd,
		depsCmd,
		repairMapCmd,
		impactCmd,
		deadConfigCmd,
		orphanCmd,
		configMergeCmd,
		ghostFileCmd,
		explainDiffCmd,
		humanReadableCmd,
		manualTraceCmd,
		initCmd,
		addCmd,
		listCmd,
		statusCmd,
		statsCmd,
		cemeteryCmd,
		archiveCmd,
		unarchiveCmd,
		forgetCmd,
		setThresholdCmd,
		migrateToCentralCmd,
	)

	return root
}

// Execute runs the CLI and returns any error.
func Execute() error {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		return err
	}
	return nil
}

// RootCommand returns a fresh root cobra.Command for testing.
func RootCommand() *cobra.Command {
	return newRootCmd()
}
