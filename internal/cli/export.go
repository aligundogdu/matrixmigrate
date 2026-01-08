package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var exportCmd = &cobra.Command{
	Use:   "export [assets|memberships|messages]",
	Short: "Export data from Mattermost",
	Long: `Export data from Mattermost database.

Available subcommands:
  assets       - Export users, teams, and channels
  memberships  - Export team and channel memberships
  messages     - Export all messages (posts)`,
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

var exportMessagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Export all messages from Mattermost",
	Long:  `Export all messages (posts) from Mattermost database to a compressed JSON file.`,
	RunE:  runExportMessages,
}

func init() {
	exportCmd.AddCommand(exportAssetsCmd)
	exportCmd.AddCommand(exportMembershipsCmd)
	exportCmd.AddCommand(exportMessagesCmd)
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

	result, err := orch.ExportAssets(progress)
	if err != nil {
		return err
	}

	printSuccess(i18n.T("messages.file_saved", result.OutputFile))
	printInfo(fmt.Sprintf("  Users: %d, Teams: %d, Channels: %d", 
		result.UsersExported, result.TeamsExported, result.ChannelsExported))
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

	result, err := orch.ExportMemberships(progress)
	if err != nil {
		return err
	}

	printSuccess(i18n.T("messages.file_saved", result.OutputFile))
	printInfo(fmt.Sprintf("  Team memberships: %d, Channel memberships: %d", 
		result.TeamMembershipsExported, result.ChannelMembershipsExported))
	printSuccess(i18n.T("messages.step_completed", "export_memberships"))

	return nil
}

func runExportMessages(cmd *cobra.Command, args []string) error {
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
	canRun, reason := state.CanRunStep(migration.StepExportMessages)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Connect to Mattermost
	printInfo(i18n.T("progress.connecting", "Mattermost"))
	if err := orch.ConnectMattermost(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Mattermost"))

	// Export messages
	printInfo("Exporting messages...")
	progress := func(stage string, current, total int, item string) {
		if total > 0 {
			printProgress("%s: %d/%d", stage, current, total)
		} else {
			printProgress("%s...", stage)
		}
	}

	result, err := orch.ExportMessages(progress)
	if err != nil {
		return err
	}

	printSuccess(i18n.T("messages.file_saved", result.OutputFile))
	printInfo(fmt.Sprintf("  Messages exported: %d", result.MessagesExported))
	printSuccess(i18n.T("messages.step_completed", "export_messages"))

	return nil
}


