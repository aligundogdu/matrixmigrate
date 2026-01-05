package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

var testCmd = &cobra.Command{
	Use:   "test [mattermost|matrix|all]",
	Short: "Test connections to servers",
	Long: `Test SSH and database/API connections to Mattermost and Matrix servers.

Available subcommands:
  mattermost  - Test Mattermost SSH and database connection
  matrix      - Test Matrix SSH and API connection
  all         - Test all connections (default)`,
	RunE: runTestAll,
}

var testMattermostCmd = &cobra.Command{
	Use:   "mattermost",
	Short: "Test Mattermost connection",
	Long:  `Test SSH connection and PostgreSQL database access to Mattermost server.`,
	RunE:  runTestMattermostDetailed,
}

var testMatrixCmd = &cobra.Command{
	Use:   "matrix",
	Short: "Test Matrix connection",
	Long:  `Test SSH connection and Synapse Admin API access to Matrix server.`,
	RunE:  runTestMatrixDetailed,
}

var testAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Test all connections",
	Long:  `Test both Mattermost and Matrix connections.`,
	RunE:  runTestAll,
}

func init() {
	testCmd.AddCommand(testMattermostCmd)
	testCmd.AddCommand(testMatrixCmd)
	testCmd.AddCommand(testAllCmd)
}

// Styles for test output
var (
	testHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginTop(1).
			MarginBottom(1)

	testSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#06B6D4")).
				MarginTop(1)

	testPassedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	testFailedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	testWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B"))

	testSkippedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280"))

	testRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3B82F6"))

	testDetailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			PaddingLeft(4)

	testErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			PaddingLeft(4)
)

func getStepStyle(status migration.TestStatus) lipgloss.Style {
	switch status {
	case migration.TestPassed:
		return testPassedStyle
	case migration.TestFailed:
		return testFailedStyle
	case migration.TestWarning:
		return testWarningStyle
	case migration.TestSkipped:
		return testSkippedStyle
	case migration.TestRunning:
		return testRunningStyle
	default:
		return testSkippedStyle
	}
}

func printStep(step *migration.TestStep) {
	icon := migration.GetTestStatusIcon(step.Status)
	style := getStepStyle(step.Status)
	
	fmt.Printf("  %s %s\n", style.Render(icon), step.Description)
	
	if step.Details != "" && step.Status != migration.TestFailed {
		fmt.Println(testDetailStyle.Render("└─ " + step.Details))
	}
	
	if step.Error != "" {
		fmt.Println(testErrorStyle.Render("└─ Error: " + step.Error))
	}
}

func runTestAll(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(testHeaderStyle.Render("Connection Test"))
		fmt.Println()
		fmt.Println(testSectionStyle.Render("📋 Configuration"))
		fmt.Printf("  %s Configuration file\n", testFailedStyle.Render("✗"))
		fmt.Println(testErrorStyle.Render("└─ Error: " + err.Error()))
		return nil
	}

	locale := i18n.Current()
	
	fmt.Println(testHeaderStyle.Render("Connection Test"))
	
	// Run all tests with callback
	result := migration.RunConnectionTests(cfg, func(server string, step *migration.TestStep) {
		// We'll print after collecting all results
	})
	
	// Print Config section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("📋 Configuration"))
	for _, step := range result.ConfigSteps {
		printStep(&step)
	}
	
	// Print Mattermost section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("🗄️  Mattermost"))
	if len(result.MattermostSteps) == 0 {
		fmt.Println(testSkippedStyle.Render("  ⊘ Not configured"))
	} else {
		for _, step := range result.MattermostSteps {
			printStep(&step)
		}
	}
	
	// Print Matrix section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("🔷 Matrix"))
	if len(result.MatrixSteps) == 0 {
		fmt.Println(testSkippedStyle.Render("  ⊘ Not configured"))
	} else {
		for _, step := range result.MatrixSteps {
			printStep(&step)
		}
	}
	
	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	
	if result.AllPassed {
		fmt.Println(testPassedStyle.Render("✓ " + locale.Test.AllPassed))
	} else {
		fmt.Println(testFailedStyle.Render("✗ " + locale.Test.SomeFailed))
	}
	fmt.Println()

	return nil
}

func runTestMattermostDetailed(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(testHeaderStyle.Render("Mattermost Connection Test"))
		fmt.Println()
		fmt.Printf("  %s Configuration file\n", testFailedStyle.Render("✗"))
		fmt.Println(testErrorStyle.Render("└─ Error: " + err.Error()))
		return nil
	}

	locale := i18n.Current()
	
	fmt.Println(testHeaderStyle.Render("Mattermost Connection Test"))
	
	// Run tests
	result := migration.RunConnectionTests(cfg, nil)
	
	// Print Config section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("📋 Configuration"))
	for _, step := range result.ConfigSteps {
		printStep(&step)
	}
	
	// Print Mattermost section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("🗄️  Mattermost"))
	if len(result.MattermostSteps) == 0 {
		fmt.Println(testSkippedStyle.Render("  ⊘ Not configured"))
	} else {
		for _, step := range result.MattermostSteps {
			printStep(&step)
		}
	}
	
	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	
	// Check only mattermost steps
	allPassed := true
	for _, step := range result.MattermostSteps {
		if step.Status == migration.TestFailed {
			allPassed = false
			break
		}
	}
	
	if allPassed && len(result.MattermostSteps) > 0 {
		fmt.Println(testPassedStyle.Render("✓ " + locale.Test.AllPassed))
	} else if len(result.MattermostSteps) == 0 {
		fmt.Println(testWarningStyle.Render("⚠ Mattermost not configured"))
	} else {
		fmt.Println(testFailedStyle.Render("✗ " + locale.Test.SomeFailed))
	}
	fmt.Println()

	return nil
}

func runTestMatrixDetailed(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(testHeaderStyle.Render("Matrix Connection Test"))
		fmt.Println()
		fmt.Printf("  %s Configuration file\n", testFailedStyle.Render("✗"))
		fmt.Println(testErrorStyle.Render("└─ Error: " + err.Error()))
		return nil
	}

	locale := i18n.Current()
	
	fmt.Println(testHeaderStyle.Render("Matrix Connection Test"))
	
	// Run tests
	result := migration.RunConnectionTests(cfg, nil)
	
	// Print Config section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("📋 Configuration"))
	for _, step := range result.ConfigSteps {
		printStep(&step)
	}
	
	// Print Matrix section
	fmt.Println()
	fmt.Println(testSectionStyle.Render("🔷 Matrix"))
	if len(result.MatrixSteps) == 0 {
		fmt.Println(testSkippedStyle.Render("  ⊘ Not configured"))
	} else {
		for _, step := range result.MatrixSteps {
			printStep(&step)
		}
	}
	
	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	
	// Check only matrix steps
	allPassed := true
	for _, step := range result.MatrixSteps {
		if step.Status == migration.TestFailed {
			allPassed = false
			break
		}
	}
	
	if allPassed && len(result.MatrixSteps) > 0 {
		fmt.Println(testPassedStyle.Render("✓ " + locale.Test.AllPassed))
	} else if len(result.MatrixSteps) == 0 {
		fmt.Println(testWarningStyle.Render("⚠ Matrix not configured"))
	} else {
		fmt.Println(testFailedStyle.Render("✗ " + locale.Test.SomeFailed))
	}
	fmt.Println()

	return nil
}
