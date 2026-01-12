package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
	noColor   bool
	quiet     bool

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
and improve output quality through V1/V2/V3 validation protocol.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return initConfig()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func SetVersion(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: .quorum.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "auto",
		"log format (auto, text, json)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false,
		"suppress non-essential output")

	// Bind flags to viper (errors are nil when flag exists)
	_ = viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".quorum")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/quorum")
	}

	viper.SetEnvPrefix("QUORUM")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	return nil
}
