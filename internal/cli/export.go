package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var exportCmd = &cobra.Command{
	Use:   "export [assets|memberships]",
	Short: "Export data from Mattermost",
	Long: `Export data from Mattermost database.

Available subcommands:
  assets       - Export users, teams, and channels
  memberships  - Export team and channel memberships`,
}

var exportAssetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Export users, teams, and channels from Mattermost",
	Long:  `Export users, teams, and channels from Mattermost database to a compressed JSON file.`,
	RunE:  runExportAssets,
}

var exportMembershipsCmd = &cobra.Command{
	Use:   "memberships",
	Short: "Export team and channel memberships from Mattermost",
	Long:  `Export team and channel memberships from Mattermost database to a compressed JSON file.`,
	RunE:  runExportMemberships,
}

func init() {
	exportCmd.AddCommand(exportAssetsCmd)
	exportCmd.AddCommand(exportMembershipsCmd)
}

func runExportAssets(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	printInfo(i18n.T("messages.migration_started"))

	// Create orchestrator
	orch, err := migration.NewOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Connect to Mattermost
	printInfo(i18n.T("progress.connecting", "Mattermost"))
	if err := orch.ConnectMattermost(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Mattermost"))

	// Export assets
	printInfo(i18n.T("progress.exporting"))
	progress := func(stage string, current, total int, item string) {
		if total > 0 {
			printProgress("%s: %d/%d", stage, current, total)
		} else {
			printProgress("%s...", stage)
		}
	}

	if err := orch.ExportAssets(progress); err != nil {
		return err
	}

	// Get the output file path from state
	state := orch.GetState()
	step := state.GetStep(migration.StepExportAssets)
	
	printSuccess(i18n.T("messages.file_saved", step.OutputFile))
	printSuccess(i18n.T("messages.step_completed", "export_assets"))

	return nil
}

func runExportMemberships(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	printInfo(i18n.T("messages.migration_started"))

	// Create orchestrator
	orch, err := migration.NewOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Check prerequisites
	state := orch.GetState()
	canRun, reason := state.CanRunStep(migration.StepExportMemberships)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Connect to Mattermost
	printInfo(i18n.T("progress.connecting", "Mattermost"))
	if err := orch.ConnectMattermost(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Mattermost"))

	// Export memberships
	printInfo(i18n.T("progress.exporting"))
	progress := func(stage string, current, total int, item string) {
		if total > 0 {
			printProgress("%s: %d/%d", stage, current, total)
		} else {
			printProgress("%s...", stage)
		}
	}

	if err := orch.ExportMemberships(progress); err != nil {
		return err
	}

	// Get the output file path from state
	step := state.GetStep(migration.StepExportMemberships)
	
	printSuccess(i18n.T("messages.file_saved", step.OutputFile))
	printSuccess(i18n.T("messages.step_completed", "export_memberships"))

	return nil
}

