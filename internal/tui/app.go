package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aligundogdu/matrixmigrate/internal/config"
	"github.com/aligundogdu/matrixmigrate/internal/i18n"
	"github.com/aligundogdu/matrixmigrate/internal/migration"
)

// View represents different screens in the app
type View int

const (
	ViewMenu View = iota
	ViewExportAssets
	ViewImportAssets
	ViewExportMemberships
	ViewImportMemberships
	ViewTestConnection
	ViewStatus
	ViewSettings
	ViewProgress
	ViewError
	ViewSuccess
)

// Model is the main application model
type Model struct {
	// App state
	config       *config.Config
	orchestrator *migration.Orchestrator
	view         View
	previousView View

	// UI components
	menuItems    []MenuItem
	menuIndex    int
	spinner      spinner.Model
	width        int
	height       int

	// Progress state
	progressStage   string
	progressCurrent int
	progressTotal   int
	progressItem    string

	// Test results
	testResult *migration.ConnectionTestResult
	testDone   bool

	// Messages
	errorMessage   string
	successMessage string

	// Operation result for detailed stats
	operationResult *migration.OperationResult

	// Quitting
	quitting bool
}

// MenuItem represents a menu item
type MenuItem struct {
	Title    string
	Desc     string
	View     View
	Disabled bool
	Action   func() tea.Cmd
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// NewModel creates a new application model
func NewModel(cfg *config.Config) (Model, error) {
	// Create orchestrator
	orchestrator, err := migration.NewOrchestrator(cfg)
	if err != nil {
		return Model{}, fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	m := Model{
		config:       cfg,
		orchestrator: orchestrator,
		view:         ViewMenu,
		spinner:      s,
		width:        80,
		height:       24,
	}

	// Initialize menu items
	m.menuItems = m.createMenuItems()

	return m, nil
}

// createMenuItems creates the main menu items
func (m *Model) createMenuItems() []MenuItem {
	locale := i18n.Current()
	state := m.orchestrator.GetState()

	// Check which steps can be run
	canExportAssets, _ := state.CanRunStep(migration.StepExportAssets)
	canImportAssets, _ := state.CanRunStep(migration.StepImportAssets)
	canExportMemberships, _ := state.CanRunStep(migration.StepExportMemberships)
	canImportMemberships, _ := state.CanRunStep(migration.StepImportMemberships)

	return []MenuItem{
		{
			Title:    locale.Menu.ExportAssets,
			Desc:     "Export users, teams, and channels from Mattermost",
			View:     ViewExportAssets,
			Disabled: !canExportAssets,
		},
		{
			Title:    locale.Menu.ImportAssets,
			Desc:     "Import assets to Matrix",
			View:     ViewImportAssets,
			Disabled: !canImportAssets,
		},
		{
			Title:    locale.Menu.ExportMemberships,
			Desc:     "Export team and channel memberships",
			View:     ViewExportMemberships,
			Disabled: !canExportMemberships,
		},
		{
			Title:    locale.Menu.ImportMemberships,
			Desc:     "Apply memberships in Matrix",
			View:     ViewImportMemberships,
			Disabled: !canImportMemberships,
		},
		{
			Title: locale.Menu.TestConnection,
			Desc:  "Test Mattermost and Matrix connections",
			View:  ViewTestConnection,
		},
		{
			Title: locale.Menu.Status,
			Desc:  "View migration status",
			View:  ViewStatus,
		},
		{
			Title: locale.Menu.Quit,
			Desc:  "Exit the application",
			View:  ViewMenu,
			Action: func() tea.Cmd {
				return tea.Quit
			},
		},
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progressMsg:
		m.progressStage = msg.stage
		m.progressCurrent = msg.current
		m.progressTotal = msg.total
		m.progressItem = msg.item
		return m, nil

	case operationCompleteMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			m.view = ViewError
		} else {
			m.successMessage = msg.message
			m.operationResult = msg.result
			m.view = ViewSuccess
		}
		// Refresh menu items
		m.menuItems = m.createMenuItems()
		return m, nil

	case testCompleteMsg:
		m.testResult = msg.result
		m.testDone = true
		m.view = ViewTestConnection
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.view == ViewMenu {
			m.quitting = true
			return m, tea.Quit
		}
		// Go back to menu
		m.view = ViewMenu
		return m, nil

	case "up", "k":
		if m.view == ViewMenu {
			m.menuIndex--
			if m.menuIndex < 0 {
				m.menuIndex = len(m.menuItems) - 1
			}
		}
		return m, nil

	case "down", "j":
		if m.view == ViewMenu {
			m.menuIndex++
			if m.menuIndex >= len(m.menuItems) {
				m.menuIndex = 0
			}
		}
		return m, nil

	case "enter", " ":
		if m.view == ViewMenu {
			item := m.menuItems[m.menuIndex]
			if item.Disabled {
				return m, nil
			}
			if item.Action != nil {
				return m, item.Action()
			}
			m.previousView = m.view
			m.view = item.View
			return m, m.handleViewChange(item.View)
		}
		if m.view == ViewError || m.view == ViewSuccess {
			m.view = ViewMenu
			return m, nil
		}
		return m, nil

	case "esc":
		if m.view != ViewMenu {
			m.view = ViewMenu
		}
		return m, nil
	}

	return m, nil
}

// handleViewChange returns commands for view transitions
func (m *Model) handleViewChange(view View) tea.Cmd {
	switch view {
	case ViewExportAssets:
		return m.runExportAssets()
	case ViewImportAssets:
		return m.runImportAssets()
	case ViewExportMemberships:
		return m.runExportMemberships()
	case ViewImportMemberships:
		return m.runImportMemberships()
	case ViewTestConnection:
		return m.runTestConnection()
	case ViewStatus:
		// Status view doesn't need a command
		return nil
	}
	return nil
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	switch m.view {
	case ViewMenu:
		return m.renderMenu()
	case ViewProgress:
		return m.renderProgress()
	case ViewStatus:
		return m.renderStatus()
	case ViewError:
		return m.renderError()
	case ViewSuccess:
		return m.renderSuccess()
	case ViewTestConnection:
		return m.renderTestConnection()
	case ViewExportAssets, ViewImportAssets, ViewExportMemberships, ViewImportMemberships:
		return m.renderProgress()
	default:
		return m.renderMenu()
	}
}

// renderMenu renders the main menu
func (m Model) renderMenu() string {
	locale := i18n.Current()

	// Header
	header := LogoStyle.Render(`
 __  __       _        _      __  __ _                 _       
|  \/  | __ _| |_ _ __(_)_  _|  \/  (_) __ _ _ __ __ _| |_ ___ 
| |\/| |/ _` + "`" + ` | __| '__| \ \/ /| |\/| | |/ _` + "`" + ` | '__/ _` + "`" + ` | __/ _ \
| |  | | (_| | |_| |  | |>  < | |  | | | (_| | | | (_| | ||  __/
|_|  |_|\__,_|\__|_|  |_/_/\_\|_|  |_|_|\__, |_|  \__,_|\__\___|
                                        |___/                   `)

	subtitle := SubtitleStyle.Render(locale.App.Description)

	// Menu items
	var menuContent string
	for i, item := range m.menuItems {
		cursor := "  "
		style := MenuItemStyle
		descStyle := MenuItemDescStyle
		if i == m.menuIndex {
			cursor = IconArrow + " "
			style = MenuItemSelectedStyle
			descStyle = MenuItemDescSelectedStyle
		}
		if item.Disabled {
			style = MenuItemDisabledStyle
			descStyle = MenuItemDescStyle
		}

		menuContent += cursor + style.Render(item.Title) + "\n"
		if i == m.menuIndex && item.Desc != "" {
			menuContent += descStyle.Render("└─ "+item.Desc) + "\n"
		}
	}

	// Help
	help := HelpStyle.Render("↑/↓: navigate • enter: select • q: quit")

	// Combine
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		subtitle,
		"",
		BoxStyle.Render(TitleStyle.Render(locale.Menu.Title)+"\n\n"+menuContent),
		help,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// renderProgress renders the progress view
func (m Model) renderProgress() string {
	locale := i18n.Current()

	// Title based on current operation
	var title string
	switch m.view {
	case ViewExportAssets:
		title = locale.Menu.ExportAssets
	case ViewImportAssets:
		title = locale.Menu.ImportAssets
	case ViewExportMemberships:
		title = locale.Menu.ExportMemberships
	case ViewImportMemberships:
		title = locale.Menu.ImportMemberships
	case ViewTestConnection:
		title = locale.Menu.TestConnection
	default:
		title = locale.Progress.Exporting
	}

	// Spinner
	spinner := m.spinner.View()

	// Progress info
	var progressInfo string
	if m.progressTotal > 0 {
		percentage := float64(m.progressCurrent) / float64(m.progressTotal) * 100
		bar := renderProgressBar(int(percentage), 40)
		progressInfo = fmt.Sprintf("%s\n\n%s %.0f%% (%d/%d)",
			m.progressStage,
			bar,
			percentage,
			m.progressCurrent,
			m.progressTotal,
		)
		if m.progressItem != "" {
			progressInfo += "\n" + MutedStyle.Render(m.progressItem)
		}
	} else {
		progressInfo = m.progressStage
	}

	content := BoxStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render(title),
			"",
			spinner+" "+progressInfo,
		),
	)

	help := HelpStyle.Render("Please wait...")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help))
}

// renderProgressBar renders a simple progress bar
func renderProgressBar(percent, width int) string {
	filled := width * percent / 100
	empty := width - filled

	bar := ProgressBarStyle.Render(repeatStr("█", filled)) +
		MutedStyle.Render(repeatStr("░", empty))

	return bar
}

// repeatStr repeats a string n times
func repeatStr(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// renderStatus renders the status view
func (m Model) renderStatus() string {
	locale := i18n.Current()
	state := m.orchestrator.GetState()

	// Build status table
	steps := []migration.StepName{
		migration.StepExportAssets,
		migration.StepImportAssets,
		migration.StepExportMemberships,
		migration.StepImportMemberships,
	}

	var rows string
	for _, stepName := range steps {
		step := state.GetStep(stepName)
		icon := GetStatusIcon(string(step.Status))
		style := GetStatusStyle(string(step.Status))

		name := string(stepName)
		status := style.Render(icon + " " + string(step.Status))

		rows += fmt.Sprintf("  %-25s %s\n", name, status)
	}

	content := BoxStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render(locale.Status.Title),
			"",
			rows,
		),
	)

	help := HelpStyle.Render("Press esc or q to go back")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help))
}

// renderError renders the error view
func (m Model) renderError() string {
	content := ErrorBoxStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			ErrorStyle.Render(IconCross+" Error"),
			"",
			m.errorMessage,
		),
	)

	help := HelpStyle.Render("Press enter to continue")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help))
}

// renderSuccess renders the success view with detailed stats
func (m Model) renderSuccess() string {
	var sections []string

	// Title
	sections = append(sections, SuccessStyle.Render(IconCheck+" Success"))
	sections = append(sections, "")
	sections = append(sections, m.successMessage)

	// Show detailed stats if available
	if m.operationResult != nil {
		sections = append(sections, "")
		sections = append(sections, SubtitleStyle.Render("─────────────────────────"))
		sections = append(sections, "")

		r := m.operationResult

		// Export stats
		if r.UsersExported > 0 || r.TeamsExported > 0 || r.ChannelsExported > 0 {
			sections = append(sections, SubtitleStyle.Render("📤 Exported:"))
			if r.UsersExported > 0 {
				sections = append(sections, fmt.Sprintf("   • Users: %d", r.UsersExported))
			}
			if r.TeamsExported > 0 {
				sections = append(sections, fmt.Sprintf("   • Teams: %d", r.TeamsExported))
			}
			if r.ChannelsExported > 0 {
				sections = append(sections, fmt.Sprintf("   • Channels: %d", r.ChannelsExported))
			}
			sections = append(sections, "")
		}

		// Membership export stats
		if r.TeamMembershipsExported > 0 || r.ChannelMembershipsExported > 0 {
			sections = append(sections, SubtitleStyle.Render("📤 Memberships Exported:"))
			if r.TeamMembershipsExported > 0 {
				sections = append(sections, fmt.Sprintf("   • Team memberships: %d", r.TeamMembershipsExported))
			}
			if r.ChannelMembershipsExported > 0 {
				sections = append(sections, fmt.Sprintf("   • Channel memberships: %d", r.ChannelMembershipsExported))
			}
			sections = append(sections, "")
		}

		// Import stats - Users
		if r.UsersCreated > 0 || r.UsersSkipped > 0 || r.UsersFailed > 0 {
			sections = append(sections, SubtitleStyle.Render("👥 Users:"))
			if r.UsersCreated > 0 {
				sections = append(sections, SuccessStyle.Render(fmt.Sprintf("   ✓ Created: %d", r.UsersCreated)))
			}
			if r.UsersSkipped > 0 {
				sections = append(sections, DimStyle.Render(fmt.Sprintf("   ⊘ Skipped: %d", r.UsersSkipped)))
			}
			if r.UsersFailed > 0 {
				sections = append(sections, ErrorStyle.Render(fmt.Sprintf("   ✗ Failed: %d", r.UsersFailed)))
			}
			sections = append(sections, "")
		}

		// Import stats - Spaces
		if r.SpacesCreated > 0 || r.SpacesSkipped > 0 || r.SpacesFailed > 0 {
			sections = append(sections, SubtitleStyle.Render("🏠 Spaces:"))
			if r.SpacesCreated > 0 {
				sections = append(sections, SuccessStyle.Render(fmt.Sprintf("   ✓ Created: %d", r.SpacesCreated)))
			}
			if r.SpacesSkipped > 0 {
				sections = append(sections, DimStyle.Render(fmt.Sprintf("   ⊘ Skipped: %d", r.SpacesSkipped)))
			}
			if r.SpacesFailed > 0 {
				sections = append(sections, ErrorStyle.Render(fmt.Sprintf("   ✗ Failed: %d", r.SpacesFailed)))
			}
			sections = append(sections, "")
		}

		// Import stats - Rooms
		if r.RoomsCreated > 0 || r.RoomsSkipped > 0 || r.RoomsFailed > 0 || r.RoomsLinked > 0 {
			sections = append(sections, SubtitleStyle.Render("💬 Rooms:"))
			if r.RoomsCreated > 0 {
				sections = append(sections, SuccessStyle.Render(fmt.Sprintf("   ✓ Created: %d", r.RoomsCreated)))
			}
			if r.RoomsLinked > 0 {
				sections = append(sections, SuccessStyle.Render(fmt.Sprintf("   ✓ Linked to spaces: %d", r.RoomsLinked)))
			}
			if r.RoomsSkipped > 0 {
				sections = append(sections, DimStyle.Render(fmt.Sprintf("   ⊘ Skipped: %d", r.RoomsSkipped)))
			}
			if r.RoomsFailed > 0 {
				sections = append(sections, ErrorStyle.Render(fmt.Sprintf("   ✗ Failed: %d", r.RoomsFailed)))
			}
			sections = append(sections, "")
		}

		// Membership import stats
		if r.MembersAdded > 0 || r.MembersSkipped > 0 || r.MembersFailed > 0 {
			sections = append(sections, SubtitleStyle.Render("👤 Memberships:"))
			if r.MembersAdded > 0 {
				sections = append(sections, SuccessStyle.Render(fmt.Sprintf("   ✓ Added: %d", r.MembersAdded)))
			}
			if r.MembersSkipped > 0 {
				sections = append(sections, DimStyle.Render(fmt.Sprintf("   ⊘ Skipped: %d", r.MembersSkipped)))
			}
			if r.MembersFailed > 0 {
				sections = append(sections, ErrorStyle.Render(fmt.Sprintf("   ✗ Failed: %d", r.MembersFailed)))
			}
			sections = append(sections, "")
		}

		// Output file
		if r.OutputFile != "" {
			sections = append(sections, DimStyle.Render("📁 Output: "+r.OutputFile))
		}
	}

	content := SuccessBoxStyle.Width(50).Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)

	help := HelpStyle.Render("Press enter to continue")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help))
}

// renderTestConnection renders detailed test results
func (m Model) renderTestConnection() string {
	locale := i18n.Current()

	if !m.testDone || m.testResult == nil {
		// Still running
		content := BoxStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				m.spinner.View(),
				"",
				locale.Test.Testing,
			),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Build test results
	var sections []string

	// Title
	title := TitleStyle.Render(locale.Test.Title)
	sections = append(sections, title)
	sections = append(sections, "")

	// Config section
	if len(m.testResult.ConfigSteps) > 0 {
		configTitle := SubtitleStyle.Render("📋 " + locale.Test.ConfigSection)
		sections = append(sections, configTitle)
		for _, step := range m.testResult.ConfigSteps {
			sections = append(sections, m.formatTestStep(&step))
		}
		sections = append(sections, "")
	}

	// Mattermost section
	mmTitle := SubtitleStyle.Render("🗄️ " + locale.Test.MattermostSection)
	sections = append(sections, mmTitle)
	if len(m.testResult.MattermostSteps) == 0 {
		sections = append(sections, DimStyle.Render("   No tests run"))
	} else {
		for _, step := range m.testResult.MattermostSteps {
			sections = append(sections, m.formatTestStep(&step))
		}
	}
	sections = append(sections, "")

	// Matrix section
	mxTitle := SubtitleStyle.Render("🔷 " + locale.Test.MatrixSection)
	sections = append(sections, mxTitle)
	if len(m.testResult.MatrixSteps) == 0 {
		sections = append(sections, DimStyle.Render("   No tests run"))
	} else {
		for _, step := range m.testResult.MatrixSteps {
			sections = append(sections, m.formatTestStep(&step))
		}
	}
	sections = append(sections, "")

	// Overall result
	if m.testResult.AllPassed {
		sections = append(sections, SuccessStyle.Render(IconCheck+" "+locale.Test.AllPassed))
	} else {
		sections = append(sections, ErrorStyle.Render(IconCross+" "+locale.Test.SomeFailed))
	}

	content := BoxStyle.Width(70).Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)

	help := HelpStyle.Render("Press esc or q to go back")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help))
}

// formatTestStep formats a single test step for display
func (m Model) formatTestStep(step *migration.TestStep) string {
	icon := migration.GetTestStatusIcon(step.Status)
	var style lipgloss.Style

	switch step.Status {
	case migration.TestPassed:
		style = SuccessStyle
	case migration.TestFailed:
		style = ErrorStyle
	case migration.TestSkipped:
		style = DimStyle
	case migration.TestWarning:
		style = WarningStyle
	case migration.TestRunning:
		style = PrimaryStyle
	default:
		style = DimStyle
	}

	line := fmt.Sprintf("   %s %s", style.Render(icon), step.Description)

	if step.Details != "" && step.Status == migration.TestPassed {
		line += DimStyle.Render(" (" + step.Details + ")")
	}

	if step.Error != "" {
		line += "\n      " + ErrorStyle.Render("└─ " + step.Error)
	}

	return line
}

// Message types for async operations
type progressMsg struct {
	stage   string
	current int
	total   int
	item    string
}

type operationCompleteMsg struct {
	message string
	err     error
	result  *migration.OperationResult
}

// Run commands for various operations
func (m *Model) runExportAssets() tea.Cmd {
	return func() tea.Msg {
		m.view = ViewProgress
		m.progressStage = "Connecting to Mattermost..."

		// Connect to Mattermost
		if err := m.orchestrator.ConnectMattermost(); err != nil {
			return operationCompleteMsg{err: err}
		}

		// Run export
		progress := func(stage string, current, total int, item string) {
			// Note: In a real implementation, we'd use program.Send()
			m.progressStage = stage
			m.progressCurrent = current
			m.progressTotal = total
			m.progressItem = item
		}

		result, err := m.orchestrator.ExportAssets(progress)
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{message: "Assets exported successfully!", result: result}
	}
}

func (m *Model) runImportAssets() tea.Cmd {
	return func() tea.Msg {
		m.view = ViewProgress
		m.progressStage = "Connecting to Matrix..."

		// Connect to Matrix
		if err := m.orchestrator.ConnectMatrix(); err != nil {
			return operationCompleteMsg{err: err}
		}

		// Run import
		progress := func(stage string, current, total int, item string) {
			m.progressStage = stage
			m.progressCurrent = current
			m.progressTotal = total
			m.progressItem = item
		}

		result, err := m.orchestrator.ImportAssets(progress)
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{message: "Assets imported successfully!", result: result}
	}
}

func (m *Model) runExportMemberships() tea.Cmd {
	return func() tea.Msg {
		m.view = ViewProgress
		m.progressStage = "Connecting to Mattermost..."

		// Connect if not already
		if err := m.orchestrator.ConnectMattermost(); err != nil {
			return operationCompleteMsg{err: err}
		}

		progress := func(stage string, current, total int, item string) {
			m.progressStage = stage
			m.progressCurrent = current
			m.progressTotal = total
			m.progressItem = item
		}

		result, err := m.orchestrator.ExportMemberships(progress)
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{message: "Memberships exported successfully!", result: result}
	}
}

func (m *Model) runImportMemberships() tea.Cmd {
	return func() tea.Msg {
		m.view = ViewProgress
		m.progressStage = "Connecting to Matrix..."

		// Connect if not already
		if err := m.orchestrator.ConnectMatrix(); err != nil {
			return operationCompleteMsg{err: err}
		}

		progress := func(stage string, current, total int, item string) {
			m.progressStage = stage
			m.progressCurrent = current
			m.progressTotal = total
			m.progressItem = item
		}

		result, err := m.orchestrator.ImportMemberships(progress)
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{message: "Memberships imported successfully!", result: result}
	}
}

// testCompleteMsg signals test is complete
type testCompleteMsg struct {
	result *migration.ConnectionTestResult
}

func (m *Model) runTestConnection() tea.Cmd {
	return func() tea.Msg {
		result := migration.RunConnectionTests(m.config, nil)
		return testCompleteMsg{result: result}
	}
}

// Run starts the TUI application
func Run(cfg *config.Config) error {
	model, err := NewModel(cfg)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

