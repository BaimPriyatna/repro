// Package main is the entry point for the repro CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/internal/cli"
)

var version = "dev"

func main() {
	root := cli.RootCommand()
	root.Version = version
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version of repro",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Println(version)
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}
