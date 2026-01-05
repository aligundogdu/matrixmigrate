package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Display the current status of all migration steps.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	locale := i18n.Current()

	// Load state
	state, err := migration.LoadState(cfg.Data.StateFile)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s\n", locale.Status.Title)
	fmt.Println("  " + "─────────────────────────────────────────────────")
	fmt.Println()

	// Display each step
	steps := []struct {
		name  migration.StepName
		label string
	}{
		{migration.StepExportAssets, "1. Export Assets"},
		{migration.StepImportAssets, "2. Import Assets"},
		{migration.StepExportMemberships, "3. Export Memberships"},
		{migration.StepImportMemberships, "4. Import Memberships"},
	}

	for _, s := range steps {
		step := state.GetStep(s.name)
		statusIcon := getStatusIcon(string(step.Status))
		statusText := getStatusText(string(step.Status), locale)

		fmt.Printf("  %s %-25s %s\n", statusIcon, s.label, statusText)

		// Show additional info for completed steps
		if step.Status == migration.StatusCompleted {
			if step.OutputFile != "" {
				fmt.Printf("      └─ Output: %s\n", step.OutputFile)
			}
			if step.CompletedAt > 0 {
				completedTime := time.UnixMilli(step.CompletedAt).Format("2006-01-02 15:04:05")
				fmt.Printf("      └─ Completed: %s\n", completedTime)
			}
		}

		// Show error for failed steps
		if step.Status == migration.StatusFailed && step.ErrorMessage != "" {
			fmt.Printf("      └─ Error: %s\n", step.ErrorMessage)
		}
	}

	fmt.Println()

	// Show summary
	summary := state.Summary()
	fmt.Printf("  Summary: %d completed, %d pending, %d failed\n",
		summary.Completed, summary.Pending, summary.Failed)

	if state.IsComplete() {
		fmt.Println()
		printSuccess(locale.Messages.MigrationCompleted)
	}

	fmt.Println()

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "pending":
		return "○"
	case "in_progress":
		return "◐"
	case "completed":
		return "●"
	case "failed":
		return "✗"
	case "skipped":
		return "⊘"
	default:
		return "?"
	}
}

func getStatusText(status string, locale *i18n.Locale) string {
	switch status {
	case "pending":
		return locale.Status.Pending
	case "in_progress":
		return locale.Status.InProgress
	case "completed":
		return locale.Status.Completed
	case "failed":
		return locale.Status.Failed
	case "skipped":
		return locale.Status.Skipped
	default:
		return status
	}
}



