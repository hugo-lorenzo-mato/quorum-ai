package main

import (
	"os"

	"github.com/hugo-lorenzo-mato/quorum-ai/cmd/quorum/cmd"
)

// Version information - set by goreleaser at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Inject version info into command package
	cmd.SetVersion(version, commit, date)

	// Execute root command
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
