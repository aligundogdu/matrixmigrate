package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/config"
	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/tui"
	"github.com/aligundogdu/matrixmigrate/internal/version"
)

var (
	cfgFile  string
	language string
	batch    bool
	verbose  bool
)

var rootCmd = &cobra.Command{
	Use:     "matrixmigrate",
	Short:   "Mattermost to Matrix migration tool",
	Version: version.GetFullVersion(),
	Long: `MatrixMigrate is a CLI tool for migrating from Mattermost to Matrix Synapse.

It supports multi-step migration with resumable checkpoints, SSH tunnel connections,
and provides both interactive TUI and batch modes.

Examples:
  # Start interactive TUI
  matrixmigrate

  # Start with Turkish interface
  matrixmigrate --lang tr

  # Run in batch mode
  matrixmigrate --batch export assets

  # Test connections
  matrixmigrate test mattermost
  matrixmigrate test matrix`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize i18n
		if err := i18n.Init(language); err != nil {
			return fmt.Errorf("failed to initialize i18n: %w", err)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			// If no config and we're in TUI mode, show a message
			if !batch {
				fmt.Println(i18n.T("messages.no_config"))
				fmt.Println("Please create a config.yaml file. See config.example.yaml for reference.")
				return nil
			}
			return err
		}

		// Override language from config if not set via flag
		if language == "en" && cfg.Language != "" {
			if err := i18n.Init(cfg.Language); err != nil {
				return err
			}
		}

		// Ensure data directories exist
		if err := cfg.EnsureDataDirs(); err != nil {
			return err
		}

		// If batch mode, show help
		if batch {
			return cmd.Help()
		}

		// Start TUI
		return tui.Run(cfg)
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&language, "lang", "l", "en", "interface language (en, tr)")
	rootCmd.PersistentFlags().BoolVar(&batch, "batch", false, "run in batch mode (non-interactive)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd shows detailed version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show detailed version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetBuildInfo())
	},
}

// loadConfig is a helper to load config for subcommands
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("errors.config_not_found", cfgFile), err)
	}

	if err := cfg.EnsureDataDirs(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// printError prints an error message
func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// printSuccess prints a success message
func printSuccess(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// printInfo prints an info message
func printInfo(format string, args ...interface{}) {
	fmt.Printf("ℹ "+format+"\n", args...)
}

// printWarning prints a warning message
func printWarning(format string, args ...interface{}) {
	fmt.Printf("⚠ "+format+"\n", args...)
}

// printProgress prints a progress message
func printProgress(format string, args ...interface{}) {
	if verbose {
		fmt.Printf("  "+format+"\n", args...)
	}
}
