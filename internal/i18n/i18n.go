package i18n

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed locales/*.yaml
var localesFS embed.FS

// Locale represents all translatable strings
type Locale struct {
	App      AppStrings      `yaml:"app"`
	Menu     MenuStrings     `yaml:"menu"`
	Progress ProgressStrings `yaml:"progress"`
	Messages MessageStrings  `yaml:"messages"`
	Status   StatusStrings   `yaml:"status"`
	Errors   ErrorStrings    `yaml:"errors"`
	Test     TestStrings     `yaml:"test"`
	Help     HelpStrings     `yaml:"help"`
}

// AppStrings contains application-level strings
type AppStrings struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// MenuStrings contains menu-related strings
type MenuStrings struct {
	Title             string `yaml:"title"`
	ExportAssets      string `yaml:"export_assets"`
	ImportAssets      string `yaml:"import_assets"`
	ExportMemberships string `yaml:"export_memberships"`
	ImportMemberships string `yaml:"import_memberships"`
	TestConnection    string `yaml:"test_connection"`
	TestMattermost    string `yaml:"test_mattermost"`
	TestMatrix        string `yaml:"test_matrix"`
	Settings          string `yaml:"settings"`
	Status            string `yaml:"status"`
	Quit              string `yaml:"quit"`
	Back              string `yaml:"back"`
	Confirm           string `yaml:"confirm"`
	Cancel            string `yaml:"cancel"`
}

// ProgressStrings contains progress-related strings
type ProgressStrings struct {
	Connecting           string `yaml:"connecting"`
	Connected            string `yaml:"connected"`
	Disconnecting        string `yaml:"disconnecting"`
	Disconnected         string `yaml:"disconnected"`
	Exporting            string `yaml:"exporting"`
	ExportingUsers       string `yaml:"exporting_users"`
	ExportingTeams       string `yaml:"exporting_teams"`
	ExportingChannels    string `yaml:"exporting_channels"`
	ExportingMemberships string `yaml:"exporting_memberships"`
	Importing            string `yaml:"importing"`
	CreatingUsers        string `yaml:"creating_users"`
	CreatingSpaces       string `yaml:"creating_spaces"`
	CreatingRooms        string `yaml:"creating_rooms"`
	ApplyingMemberships  string `yaml:"applying_memberships"`
	LinkingRooms         string `yaml:"linking_rooms"`
	SavingFile           string `yaml:"saving_file"`
	LoadingFile          string `yaml:"loading_file"`
	Completed            string `yaml:"completed"`
	Failed               string `yaml:"failed"`
	Skipped              string `yaml:"skipped"`
	Retrying             string `yaml:"retrying"`
}

// MessageStrings contains general message strings
type MessageStrings struct {
	Welcome            string `yaml:"welcome"`
	ConnectionSuccess  string `yaml:"connection_success"`
	ConnectionFailed   string `yaml:"connection_failed"`
	FileSaved          string `yaml:"file_saved"`
	FileLoaded         string `yaml:"file_loaded"`
	ConfirmProceed     string `yaml:"confirm_proceed"`
	ConfirmOverwrite   string `yaml:"confirm_overwrite"`
	NoConfig           string `yaml:"no_config"`
	MigrationStarted   string `yaml:"migration_started"`
	MigrationCompleted string `yaml:"migration_completed"`
	MigrationFailed    string `yaml:"migration_failed"`
	MigrationCancelled string `yaml:"migration_cancelled"`
	StepCompleted      string `yaml:"step_completed"`
	StepFailed         string `yaml:"step_failed"`
	MappingSaved       string `yaml:"mapping_saved"`
	MappingLoaded      string `yaml:"mapping_loaded"`
	AssetsFound        string `yaml:"assets_found"`
	MembershipsFound   string `yaml:"memberships_found"`
}

// StatusStrings contains status-related strings
type StatusStrings struct {
	Title          string `yaml:"title"`
	Step           string `yaml:"step"`
	Status         string `yaml:"status"`
	Pending        string `yaml:"pending"`
	InProgress     string `yaml:"in_progress"`
	Completed      string `yaml:"completed"`
	Failed         string `yaml:"failed"`
	Skipped        string `yaml:"skipped"`
	LastRun        string `yaml:"last_run"`
	Never          string `yaml:"never"`
	ItemsProcessed string `yaml:"items_processed"`
	ItemsTotal     string `yaml:"items_total"`
	Errors         string `yaml:"errors"`
	Warnings       string `yaml:"warnings"`
}

// ErrorStrings contains error message strings
type ErrorStrings struct {
	ConfigNotFound        string `yaml:"config_not_found"`
	ConfigParseError      string `yaml:"config_parse_error"`
	ConfigValidationError string `yaml:"config_validation_error"`
	SSHConnectionFailed   string `yaml:"ssh_connection_failed"`
	SSHTunnelFailed       string `yaml:"ssh_tunnel_failed"`
	DBConnectionFailed    string `yaml:"db_connection_failed"`
	DBQueryFailed         string `yaml:"db_query_failed"`
	APIError              string `yaml:"api_error"`
	APIUnauthorized       string `yaml:"api_unauthorized"`
	APINotFound           string `yaml:"api_not_found"`
	APIRateLimited        string `yaml:"api_rate_limited"`
	FileReadError         string `yaml:"file_read_error"`
	FileWriteError        string `yaml:"file_write_error"`
	MappingNotFound       string `yaml:"mapping_not_found"`
	AssetNotFound         string `yaml:"asset_not_found"`
	UserCreationFailed    string `yaml:"user_creation_failed"`
	SpaceCreationFailed   string `yaml:"space_creation_failed"`
	RoomCreationFailed    string `yaml:"room_creation_failed"`
	InviteFailed          string `yaml:"invite_failed"`
	InvalidHomeserver     string `yaml:"invalid_homeserver"`
}

// TestStrings contains connection test strings
type TestStrings struct {
	Title             string `yaml:"title"`
	Testing           string `yaml:"testing"`
	ConfigSection     string `yaml:"config_section"`
	MattermostSection string `yaml:"mattermost_section"`
	MatrixSection     string `yaml:"matrix_section"`
	TestingConnection string `yaml:"testing_connection"`
	SSHSuccess        string `yaml:"ssh_success"`
	SSHFailed         string `yaml:"ssh_failed"`
	DBSuccess         string `yaml:"db_success"`
	DBFailed          string `yaml:"db_failed"`
	APISuccess        string `yaml:"api_success"`
	APIFailed         string `yaml:"api_failed"`
	AllPassed         string `yaml:"all_passed"`
	SomeFailed        string `yaml:"some_failed"`
}

// HelpStrings contains help text strings
type HelpStrings struct {
	Config  string `yaml:"config"`
	Lang    string `yaml:"lang"`
	Batch   string `yaml:"batch"`
	Verbose string `yaml:"verbose"`
	DryRun  string `yaml:"dry_run"`
}

var (
	currentLocale *Locale
	defaultLang   = "en"
	supportedLang = []string{"en", "tr"}
	mu            sync.RWMutex
)

// Init initializes the i18n system with the specified language
func Init(lang string) error {
	mu.Lock()
	defer mu.Unlock()

	// Validate language
	lang = strings.ToLower(strings.TrimSpace(lang))
	if !isSupported(lang) {
		lang = defaultLang
	}

	// Load locale file
	data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.yaml", lang))
	if err != nil {
		// Fallback to default language
		data, err = localesFS.ReadFile(fmt.Sprintf("locales/%s.yaml", defaultLang))
		if err != nil {
			return fmt.Errorf("failed to load locale file: %w", err)
		}
	}

	locale := &Locale{}
	if err := yaml.Unmarshal(data, locale); err != nil {
		return fmt.Errorf("failed to parse locale file: %w", err)
	}

	currentLocale = locale
	return nil
}

// isSupported checks if a language is supported
func isSupported(lang string) bool {
	for _, l := range supportedLang {
		if l == lang {
			return true
		}
	}
	return false
}

// GetSupportedLanguages returns a list of supported language codes
func GetSupportedLanguages() []string {
	return supportedLang
}

// Current returns the current locale
func Current() *Locale {
	mu.RLock()
	defer mu.RUnlock()

	if currentLocale == nil {
		// Auto-initialize with default if not initialized
		mu.RUnlock()
		_ = Init(defaultLang)
		mu.RLock()
	}

	return currentLocale
}

// T is a shorthand for getting translated strings with formatting
// Example: T("progress.exporting_users", 10, 100)
func T(key string, args ...interface{}) string {
	locale := Current()
	if locale == nil {
		return key
	}

	// Parse the key path
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return key
	}

	var value string
	switch parts[0] {
	case "app":
		value = getAppString(locale, parts[1])
	case "menu":
		value = getMenuString(locale, parts[1])
	case "progress":
		value = getProgressString(locale, parts[1])
	case "messages":
		value = getMessageString(locale, parts[1])
	case "status":
		value = getStatusString(locale, parts[1])
	case "errors":
		value = getErrorString(locale, parts[1])
	case "test":
		value = getTestString(locale, parts[1])
	case "help":
		value = getHelpString(locale, parts[1])
	default:
		return key
	}

	if value == "" {
		return key
	}

	if len(args) > 0 {
		return fmt.Sprintf(value, args...)
	}
	return value
}

func getAppString(l *Locale, key string) string {
	switch key {
	case "name":
		return l.App.Name
	case "description":
		return l.App.Description
	case "version":
		return l.App.Version
	}
	return ""
}

func getMenuString(l *Locale, key string) string {
	switch key {
	case "title":
		return l.Menu.Title
	case "export_assets":
		return l.Menu.ExportAssets
	case "import_assets":
		return l.Menu.ImportAssets
	case "export_memberships":
		return l.Menu.ExportMemberships
	case "import_memberships":
		return l.Menu.ImportMemberships
	case "test_connection":
		return l.Menu.TestConnection
	case "test_mattermost":
		return l.Menu.TestMattermost
	case "test_matrix":
		return l.Menu.TestMatrix
	case "settings":
		return l.Menu.Settings
	case "status":
		return l.Menu.Status
	case "quit":
		return l.Menu.Quit
	case "back":
		return l.Menu.Back
	case "confirm":
		return l.Menu.Confirm
	case "cancel":
		return l.Menu.Cancel
	}
	return ""
}

func getProgressString(l *Locale, key string) string {
	switch key {
	case "connecting":
		return l.Progress.Connecting
	case "connected":
		return l.Progress.Connected
	case "disconnecting":
		return l.Progress.Disconnecting
	case "disconnected":
		return l.Progress.Disconnected
	case "exporting":
		return l.Progress.Exporting
	case "exporting_users":
		return l.Progress.ExportingUsers
	case "exporting_teams":
		return l.Progress.ExportingTeams
	case "exporting_channels":
		return l.Progress.ExportingChannels
	case "exporting_memberships":
		return l.Progress.ExportingMemberships
	case "importing":
		return l.Progress.Importing
	case "creating_users":
		return l.Progress.CreatingUsers
	case "creating_spaces":
		return l.Progress.CreatingSpaces
	case "creating_rooms":
		return l.Progress.CreatingRooms
	case "applying_memberships":
		return l.Progress.ApplyingMemberships
	case "linking_rooms":
		return l.Progress.LinkingRooms
	case "saving_file":
		return l.Progress.SavingFile
	case "loading_file":
		return l.Progress.LoadingFile
	case "completed":
		return l.Progress.Completed
	case "failed":
		return l.Progress.Failed
	case "skipped":
		return l.Progress.Skipped
	case "retrying":
		return l.Progress.Retrying
	}
	return ""
}

func getMessageString(l *Locale, key string) string {
	switch key {
	case "welcome":
		return l.Messages.Welcome
	case "connection_success":
		return l.Messages.ConnectionSuccess
	case "connection_failed":
		return l.Messages.ConnectionFailed
	case "file_saved":
		return l.Messages.FileSaved
	case "file_loaded":
		return l.Messages.FileLoaded
	case "confirm_proceed":
		return l.Messages.ConfirmProceed
	case "confirm_overwrite":
		return l.Messages.ConfirmOverwrite
	case "no_config":
		return l.Messages.NoConfig
	case "migration_started":
		return l.Messages.MigrationStarted
	case "migration_completed":
		return l.Messages.MigrationCompleted
	case "migration_failed":
		return l.Messages.MigrationFailed
	case "migration_cancelled":
		return l.Messages.MigrationCancelled
	case "step_completed":
		return l.Messages.StepCompleted
	case "step_failed":
		return l.Messages.StepFailed
	case "mapping_saved":
		return l.Messages.MappingSaved
	case "mapping_loaded":
		return l.Messages.MappingLoaded
	case "assets_found":
		return l.Messages.AssetsFound
	case "memberships_found":
		return l.Messages.MembershipsFound
	}
	return ""
}

func getStatusString(l *Locale, key string) string {
	switch key {
	case "title":
		return l.Status.Title
	case "step":
		return l.Status.Step
	case "status":
		return l.Status.Status
	case "pending":
		return l.Status.Pending
	case "in_progress":
		return l.Status.InProgress
	case "completed":
		return l.Status.Completed
	case "failed":
		return l.Status.Failed
	case "skipped":
		return l.Status.Skipped
	case "last_run":
		return l.Status.LastRun
	case "never":
		return l.Status.Never
	case "items_processed":
		return l.Status.ItemsProcessed
	case "items_total":
		return l.Status.ItemsTotal
	case "errors":
		return l.Status.Errors
	case "warnings":
		return l.Status.Warnings
	}
	return ""
}

func getErrorString(l *Locale, key string) string {
	switch key {
	case "config_not_found":
		return l.Errors.ConfigNotFound
	case "config_parse_error":
		return l.Errors.ConfigParseError
	case "config_validation_error":
		return l.Errors.ConfigValidationError
	case "ssh_connection_failed":
		return l.Errors.SSHConnectionFailed
	case "ssh_tunnel_failed":
		return l.Errors.SSHTunnelFailed
	case "db_connection_failed":
		return l.Errors.DBConnectionFailed
	case "db_query_failed":
		return l.Errors.DBQueryFailed
	case "api_error":
		return l.Errors.APIError
	case "api_unauthorized":
		return l.Errors.APIUnauthorized
	case "api_not_found":
		return l.Errors.APINotFound
	case "api_rate_limited":
		return l.Errors.APIRateLimited
	case "file_read_error":
		return l.Errors.FileReadError
	case "file_write_error":
		return l.Errors.FileWriteError
	case "mapping_not_found":
		return l.Errors.MappingNotFound
	case "asset_not_found":
		return l.Errors.AssetNotFound
	case "user_creation_failed":
		return l.Errors.UserCreationFailed
	case "space_creation_failed":
		return l.Errors.SpaceCreationFailed
	case "room_creation_failed":
		return l.Errors.RoomCreationFailed
	case "invite_failed":
		return l.Errors.InviteFailed
	case "invalid_homeserver":
		return l.Errors.InvalidHomeserver
	}
	return ""
}

func getTestString(l *Locale, key string) string {
	switch key {
	case "title":
		return l.Test.Title
	case "testing":
		return l.Test.Testing
	case "config_section":
		return l.Test.ConfigSection
	case "mattermost_section":
		return l.Test.MattermostSection
	case "matrix_section":
		return l.Test.MatrixSection
	case "testing_connection":
		return l.Test.TestingConnection
	case "ssh_success":
		return l.Test.SSHSuccess
	case "ssh_failed":
		return l.Test.SSHFailed
	case "db_success":
		return l.Test.DBSuccess
	case "db_failed":
		return l.Test.DBFailed
	case "api_success":
		return l.Test.APISuccess
	case "api_failed":
		return l.Test.APIFailed
	case "all_passed":
		return l.Test.AllPassed
	case "some_failed":
		return l.Test.SomeFailed
	}
	return ""
}

func getHelpString(l *Locale, key string) string {
	switch key {
	case "config":
		return l.Help.Config
	case "lang":
		return l.Help.Lang
	case "batch":
		return l.Help.Batch
	case "verbose":
		return l.Help.Verbose
	case "dry_run":
		return l.Help.DryRun
	}
	return ""
}

