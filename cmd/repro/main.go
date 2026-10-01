// Package main is the entry point for the repro CLI.
package main

import (
	"os"

	"github.com/BaimPriyatna/repro/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
