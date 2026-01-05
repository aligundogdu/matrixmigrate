package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors - A modern dark theme with accent colors
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Violet
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorAccent    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	ColorSuccess = lipgloss.Color("#10B981") // Emerald
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorInfo    = lipgloss.Color("#3B82F6") // Blue

	// Neutral colors
	ColorText       = lipgloss.Color("#F9FAFB") // Gray 50
	ColorTextMuted  = lipgloss.Color("#9CA3AF") // Gray 400
	ColorBorder     = lipgloss.Color("#4B5563") // Gray 600
	ColorBackground = lipgloss.Color("#111827") // Gray 900
	ColorSurface    = lipgloss.Color("#1F2937") // Gray 800
)

// Styles
var (
	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Italic(true)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	FocusedBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// Menu styles
	MenuItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2)

	MenuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				PaddingLeft(2)

	MenuItemDisabledStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				PaddingLeft(2)

	MenuItemDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")). // Gray 500 - softer than text
				Italic(true).
				PaddingLeft(4)

	MenuItemDescSelectedStyle = lipgloss.NewStyle().
					Foreground(ColorSecondary). // Cyan for selected item desc
					Italic(true).
					PaddingLeft(4)

	// Status styles
	StatusPendingStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)

	StatusInProgressStyle = lipgloss.NewStyle().
				Foreground(ColorInfo).
				Bold(true)

	StatusCompletedStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StatusFailedStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				Bold(true)

	StatusSkippedStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// Progress bar styles
	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary)

	ProgressTextStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)

	// Button styles
	ButtonStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorSurface).
			Padding(0, 2).
			MarginRight(1)

	ButtonActiveStyle = lipgloss.NewStyle().
				Foreground(ColorBackground).
				Background(ColorPrimary).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	// Help styles
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			MarginTop(1)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorBorder).
			MarginBottom(1).
			PaddingBottom(1)

	// Logo/Banner style
	LogoStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Error styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Foreground(ColorError).
			Padding(0, 1)

	// Success styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	SuccessBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Foreground(ColorSuccess).
			Padding(0, 1)

	// Warning styles
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// Dim style for muted/skipped content
	DimStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// Primary style for running/active elements
	PrimaryStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(ColorBorder)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1)

	// Dialog styles
	DialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Width(50)
)

// Icons and symbols
const (
	IconCheck    = "✓"
	IconCross    = "✗"
	IconArrow    = "→"
	IconBullet   = "•"
	IconSpinner  = "◐"
	IconPending  = "○"
	IconProgress = "◐"
	IconDone     = "●"
	IconWarning  = "⚠"
	IconInfo     = "ℹ"
	IconFolder   = "📁"
	IconFile     = "📄"
	IconUser     = "👤"
	IconTeam     = "👥"
	IconChannel  = "💬"
	IconServer   = "🖥"
	IconKey      = "🔑"
	IconLock     = "🔒"
	IconUnlock   = "🔓"
)

// GetStatusStyle returns the appropriate style for a status
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "pending":
		return StatusPendingStyle
	case "in_progress":
		return StatusInProgressStyle
	case "completed":
		return StatusCompletedStyle
	case "failed":
		return StatusFailedStyle
	case "skipped":
		return StatusSkippedStyle
	default:
		return BaseStyle
	}
}

// GetStatusIcon returns the appropriate icon for a status
func GetStatusIcon(status string) string {
	switch status {
	case "pending":
		return IconPending
	case "in_progress":
		return IconProgress
	case "completed":
		return IconCheck
	case "failed":
		return IconCross
	case "skipped":
		return IconWarning
	default:
		return IconBullet
	}
}

