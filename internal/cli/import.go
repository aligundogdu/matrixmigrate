package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var importCmd = &cobra.Command{
	Use:   "import [assets|memberships]",
	Short: "Import data to Matrix",
	Long: `Import data to Matrix Synapse server.

Available subcommands:
  assets       - Create users, spaces, and rooms in Matrix
  memberships  - Apply team and channel memberships in Matrix`,
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

func init() {
	importCmd.AddCommand(importAssetsCmd)
	importCmd.AddCommand(importMembershipsCmd)
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


