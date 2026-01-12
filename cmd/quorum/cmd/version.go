package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("quorum-ai %s\n", appVersion)
		fmt.Printf("  commit: %s\n", appCommit)
		fmt.Printf("  built:  %s\n", appDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
