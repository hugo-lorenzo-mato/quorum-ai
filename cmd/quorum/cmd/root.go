package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
	noColor   bool
	quiet     bool
	projectID string // --project/-p flag for multi-project support

	// Version info - set via SetVersion()
	appVersion string
	appCommit  string
	appDate    string
)

var rootCmd = &cobra.Command{
	Use:   "quorum",
	Short: "Multi-agent AI orchestrator with consensus-based validation",
	Long: `quorum-ai orchestrates multiple AI agents to analyze, plan, and execute
development tasks. It uses consensus mechanisms to reduce hallucinations
and improve output quality through V1/V2/V3 validation protocol.

Running 'quorum' without arguments starts interactive chat mode.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return initConfig()
	},
	// Default to chat mode when no subcommand is provided
	RunE: runChat,
}

func Execute() error {
	return rootCmd.Execute()
}

func SetVersion(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

// GetVersion returns the application version string.
func GetVersion() string {
	return appVersion
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: .quorum/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "auto",
		"log format (auto, text, json)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false,
		"suppress non-essential output")
	rootCmd.PersistentFlags().StringVar(&projectID, "project", "",
		"project ID, name, or path to operate on (default: current directory or default project)")

	// Bind flags to viper (errors are nil when flag exists)
	_ = viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")

		// If --project flag is specified, try to resolve project path
		// Otherwise use current directory
		configPaths := []string{".quorum"}

		// Add project-specific config path if --project is specified
		if projectID != "" {
			// We'll attempt to resolve the project path
			// This is a best-effort - if it fails, we fall back to current directory
			if projectPath := tryResolveProjectPath(projectID); projectPath != "" {
				configPaths = append([]string{projectPath + "/.quorum"}, configPaths...)
			}
		}

		for _, p := range configPaths {
			viper.AddConfigPath(p)
		}
		viper.AddConfigPath("$HOME/.config/quorum")
	}

	viper.SetEnvPrefix("QUORUM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	return nil
}

// tryResolveProjectPath attempts to resolve a project path from an ID/name/path.
// This is used during config initialization, so it's best-effort and doesn't return errors.
func tryResolveProjectPath(value string) string {
	// This is a simplified version that doesn't use the registry to avoid circular deps
	// If value looks like a path, use it directly
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, ".") {
		return value
	}
	// For now, return empty - the registry-based resolution happens later
	return ""
}
