package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var importCmd = &cobra.Command{
	Use:   "import [assets|memberships|messages]",
	Short: "Import data to Matrix",
	Long: `Import data to Matrix Synapse server.

Available subcommands:
  assets       - Create users, spaces, and rooms in Matrix
  memberships  - Apply team and channel memberships in Matrix
  messages     - Import all messages to Matrix rooms`,
}

var importAssetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Import users, spaces, and rooms to Matrix",
	Long:  `Create users, spaces, and rooms in Matrix based on exported Mattermost data.`,
	RunE:  runImportAssets,
}

var importMembershipsCmd = &cobra.Command{
	Use:   "memberships",
	Short: "Apply memberships in Matrix",
	Long:  `Add users to spaces and rooms in Matrix based on Mattermost memberships.`,
	RunE:  runImportMemberships,
}

var importMessagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Import messages to Matrix",
	Long: `Import all messages to Matrix rooms.

This command requires Application Service (AS) configuration to preserve
original message timestamps. Without AS, messages will be imported with
current timestamps.

Requires: appservice.enabled=true and MATRIX_AS_TOKEN env var`,
	RunE:  runImportMessages,
}

func init() {
	importCmd.AddCommand(importAssetsCmd)
	importCmd.AddCommand(importMembershipsCmd)
	importCmd.AddCommand(importMessagesCmd)
}

func runImportAssets(cmd *cobra.Command, args []string) error {
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
	canRun, reason := state.CanRunStep(migration.StepImportAssets)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Connect to Matrix
	printInfo(i18n.T("progress.connecting", "Matrix"))
	if err := orch.ConnectMatrix(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Matrix"))

	// Import assets
	printInfo(i18n.T("progress.importing"))
	progress := func(stage string, current, total int, item string) {
		if total > 0 {
			printProgress("%s: %d/%d - %s", stage, current, total, item)
		} else {
			printProgress("%s...", stage)
		}
	}

	result, err := orch.ImportAssets(progress)
	if err != nil {
		return err
	}

	printSuccess(i18n.T("messages.mapping_saved", result.OutputFile))
	printInfo(fmt.Sprintf("  Users: created=%d, skipped=%d, failed=%d", 
		result.UsersCreated, result.UsersSkipped, result.UsersFailed))
	printInfo(fmt.Sprintf("  Spaces: created=%d, skipped=%d, failed=%d", 
		result.SpacesCreated, result.SpacesSkipped, result.SpacesFailed))
	printInfo(fmt.Sprintf("  Rooms: created=%d, skipped=%d, failed=%d, linked=%d", 
		result.RoomsCreated, result.RoomsSkipped, result.RoomsFailed, result.RoomsLinked))
	printSuccess(i18n.T("messages.step_completed", "import_assets"))

	return nil
}

func runImportMemberships(cmd *cobra.Command, args []string) error {
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
	canRun, reason := state.CanRunStep(migration.StepImportMemberships)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Connect to Matrix
	printInfo(i18n.T("progress.connecting", "Matrix"))
	if err := orch.ConnectMatrix(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Matrix"))

	// Import memberships
	printInfo(i18n.T("progress.importing"))
	progress := func(stage string, current, total int, item string) {
		if total > 0 {
			printProgress("%s: %d/%d", stage, current, total)
		} else {
			printProgress("%s...", stage)
		}
	}

	result, err := orch.ImportMemberships(progress)
	if err != nil {
		return err
	}

	printInfo(fmt.Sprintf("  Members: added=%d, skipped=%d, failed=%d", 
		result.MembersAdded, result.MembersSkipped, result.MembersFailed))
	printSuccess(i18n.T("messages.step_completed", "import_memberships"))
	printSuccess(i18n.T("messages.migration_completed"))

	return nil
}

func runImportMessages(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	printInfo(i18n.T("messages.migration_started"))

	// Check if AppService is enabled
	if !cfg.UseAppService() {
		printWarning("Application Service is not configured. Messages will be imported WITHOUT original timestamps.")
		printInfo("To preserve timestamps, configure appservice in config.yaml and set MATRIX_AS_TOKEN env var")
	}

	// Create orchestrator
	orch, err := migration.NewOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Check prerequisites
	state := orch.GetState()
	canRun, reason := state.CanRunStep(migration.StepImportMessages)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Connect to Matrix
	printInfo(i18n.T("progress.connecting", "Matrix"))
	if err := orch.ConnectMatrix(); err != nil {
		return err
	}
	printSuccess(i18n.T("progress.connected", "Matrix"))

	// Import messages
	printInfo("Importing messages...")
	progress := func(current, total int, channelName, status string) {
		percent := float64(current) / float64(total) * 100
		printProgress("Messages: %d/%d (%.1f%%) - %s", current, total, percent, status)
	}

	result, err := orch.ImportMessages(progress)
	if err != nil {
		return err
	}

	printInfo(fmt.Sprintf("  Messages: imported=%d, skipped=%d, failed=%d", 
		result.MessagesImported, result.MessagesSkipped, result.MessagesFailed))
	printInfo(fmt.Sprintf("  Replies: imported=%d, failed=%d", 
		result.RepliesImported, result.RepliesFailed))
	
	if result.MappingFile != "" {
		printSuccess(i18n.T("messages.mapping_saved", result.MappingFile))
	}
	
	printSuccess(i18n.T("messages.step_completed", "import_messages"))

	return nil
}


