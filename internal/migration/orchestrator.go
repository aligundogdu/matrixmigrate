package migration

import (
	"fmt"
	"time"

	"github.com/aligundogdu/matrixmigrate/internal/config"
	"github.com/aligundogdu/matrixmigrate/internal/mattermost"
	"github.com/aligundogdu/matrixmigrate/internal/matrix"
	"github.com/aligundogdu/matrixmigrate/internal/ssh"
	"github.com/aligundogdu/matrixmigrate/pkg/archive"
)

// Orchestrator manages the migration process
type Orchestrator struct {
	config        *config.Config
	state         *MigrationState
	tunnelManager *ssh.TunnelManager
	
	mmClient      *mattermost.Client
	mxClient      *matrix.Client
	mxToken       string // Matrix access token (from login or config)
}

// NewOrchestrator creates a new migration orchestrator
func NewOrchestrator(cfg *config.Config) (*Orchestrator, error) {
	// Load or create state
	state, err := LoadState(cfg.Data.StateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return &Orchestrator{
		config:        cfg,
		state:         state,
		tunnelManager: ssh.NewTunnelManager(),
	}, nil
}

// Close closes all connections
func (o *Orchestrator) Close() error {
	if o.mmClient != nil {
		o.mmClient.Close()
	}
	return o.tunnelManager.CloseAll()
}

// GetState returns the current migration state
func (o *Orchestrator) GetState() *MigrationState {
	return o.state
}

// SaveState saves the current state
func (o *Orchestrator) SaveState() error {
	return SaveState(o.state, o.config.Data.StateFile)
}

// ProgressCallback is called to report progress during operations
type ProgressCallback func(stage string, current, total int, item string)

// ConnectMattermost establishes connection to Mattermost
func (o *Orchestrator) ConnectMattermost() error {
	cfg := o.config.Mattermost
	passphrase := o.config.GetSSHKeyPassphrase("mattermost")
	sshPassword := o.config.GetSSHPassword("mattermost")

	// Get database credentials
	var dbHost string
	var dbPort int
	var dbUser string
	var dbPassword string
	var dbName string

	if o.config.HasManualDatabaseConfig() {
		// Use manual config
		dbHost = cfg.Database.Host
		dbPort = cfg.Database.Port
		dbUser = cfg.Database.User
		dbPassword = o.config.GetMattermostDBPassword()
		dbName = cfg.Database.Name
	} else {
		// Read from Mattermost config.json via SSH
		creds, err := mattermost.GetDatabaseCredentials(cfg.SSH, passphrase, sshPassword, cfg.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read database credentials from Mattermost config: %w", err)
		}
		dbHost = creds.Host
		dbPort = creds.Port
		dbUser = creds.User
		dbPassword = creds.Password
		dbName = creds.Database
	}

	// Get an available local port for the tunnel
	localPort, err := ssh.GetLocalPort()
	if err != nil {
		return fmt.Errorf("failed to get local port: %w", err)
	}

	// Create SSH tunnel to database
	tunnelCfg := ssh.TunnelConfig{
		SSHConfig:  cfg.SSH,
		LocalPort:  localPort,
		RemoteHost: dbHost,
		RemotePort: dbPort,
		Passphrase: passphrase,
		Password:   sshPassword,
	}

	_, err = o.tunnelManager.CreateTunnel("mattermost", tunnelCfg)
	if err != nil {
		return fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	// Build DSN using local tunnel port
	dsn := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable",
		localPort,
		dbUser,
		dbPassword,
		dbName,
	)

	// Connect to database
	client, err := mattermost.NewClient(dsn)
	if err != nil {
		o.tunnelManager.CloseTunnel("mattermost")
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	o.mmClient = client
	o.state.MattermostHost = cfg.SSH.Host
	return nil
}

// ConnectMatrix establishes connection to Matrix
func (o *Orchestrator) ConnectMatrix() error {
	cfg := o.config.Matrix
	passphrase := o.config.GetSSHKeyPassphrase("matrix")
	sshPassword := o.config.GetSSHPassword("matrix")

	// Get an available local port for the tunnel
	localPort, err := ssh.GetLocalPort()
	if err != nil {
		return fmt.Errorf("failed to get local port: %w", err)
	}

	// Create SSH tunnel to Matrix API
	tunnelCfg := ssh.TunnelConfig{
		SSHConfig:  cfg.SSH,
		LocalPort:  localPort,
		RemoteHost: "127.0.0.1",
		RemotePort: 8008, // Default Synapse port
		Passphrase: passphrase,
		Password:   sshPassword,
	}

	_, err = o.tunnelManager.CreateTunnel("matrix", tunnelCfg)
	if err != nil {
		return fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	// Use local tunnel URL
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	// Get access token (either from config or via login)
	var accessToken string
	
	if o.config.UseTokenAuth() {
		// Use provided admin token
		accessToken = o.config.GetMatrixAdminToken()
	} else {
		// Login with username/password
		password := o.config.GetMatrixPassword()
		if password == "" {
			o.tunnelManager.CloseTunnel("matrix")
			return fmt.Errorf("Matrix password not found in environment variable %s", cfg.Auth.PasswordEnv)
		}

		loginResp, err := matrix.Login(baseURL, cfg.Auth.Username, password)
		if err != nil {
			o.tunnelManager.CloseTunnel("matrix")
			return fmt.Errorf("failed to login to Matrix: %w", err)
		}
		accessToken = loginResp.AccessToken
		o.mxToken = accessToken
	}

	// Create Matrix client
	client := matrix.NewClient(baseURL, accessToken, cfg.Homeserver)

	// Test connection
	if err := client.TestConnection(); err != nil {
		o.tunnelManager.CloseTunnel("matrix")
		return fmt.Errorf("failed to connect to Matrix API: %w", err)
	}

	o.mxClient = client
	o.state.MatrixHost = cfg.SSH.Host
	return nil
}

// ExportAssets exports assets from Mattermost
func (o *Orchestrator) ExportAssets(progress ProgressCallback) error {
	if o.mmClient == nil {
		return fmt.Errorf("not connected to Mattermost")
	}

	// Check if we can run this step
	canRun, reason := o.state.CanRunStep(StepExportAssets)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Start step
	o.state.StartStep(StepExportAssets)
	if err := o.SaveState(); err != nil {
		return err
	}

	// Create exporter
	exporter := mattermost.NewExporter(o.mmClient)

	// Export callback
	var exportProgress mattermost.ExportProgressCallback
	if progress != nil {
		exportProgress = func(stage string, current, total int) {
			progress(stage, current, total, "")
			o.state.UpdateStepProgress(StepExportAssets, current, total)
		}
	}

	// Export assets
	assets, err := exporter.ExportAssets(exportProgress)
	if err != nil {
		o.state.FailStep(StepExportAssets, err)
		o.SaveState()
		return fmt.Errorf("export failed: %w", err)
	}

	// Filter to active assets only
	assets = mattermost.FilterActiveAssets(assets)

	// Generate filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("mattermost-assets-%s.json.gz", timestamp)
	filepath := o.config.Data.AssetsDir + "/" + filename

	// Save to gzipped JSON
	if err := archive.SaveGzipJSON(filepath, assets); err != nil {
		o.state.FailStep(StepExportAssets, err)
		o.SaveState()
		return fmt.Errorf("failed to save assets: %w", err)
	}

	// Complete step
	o.state.CompleteStep(StepExportAssets, filepath)
	return o.SaveState()
}

// ImportAssets imports assets to Matrix
func (o *Orchestrator) ImportAssets(progress ProgressCallback) error {
	if o.mxClient == nil {
		return fmt.Errorf("not connected to Matrix")
	}

	// Check if we can run this step
	canRun, reason := o.state.CanRunStep(StepImportAssets)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Get the asset file from previous step
	assetFile := o.state.GetStepOutputFile(StepExportAssets)
	if assetFile == "" {
		return fmt.Errorf("no asset file found from export step")
	}

	// Start step
	o.state.StartStep(StepImportAssets)
	if err := o.SaveState(); err != nil {
		return err
	}

	// Load assets
	var assets mattermost.Assets
	if err := archive.LoadGzipJSON(assetFile, &assets); err != nil {
		o.state.FailStep(StepImportAssets, err)
		o.SaveState()
		return fmt.Errorf("failed to load assets: %w", err)
	}

	// Create importer
	importer := matrix.NewImporter(o.mxClient)

	// Import callback
	var importProgress matrix.ImportProgressCallback
	if progress != nil {
		importProgress = func(stage string, current, total int, item string) {
			progress(stage, current, total, item)
			o.state.UpdateStepProgress(StepImportAssets, current, total)
		}
	}

	// Import assets
	result, err := importer.ImportAssets(&assets, importProgress)
	if err != nil {
		o.state.FailStep(StepImportAssets, err)
		o.SaveState()
		return fmt.Errorf("import failed: %w", err)
	}

	// Create mapping
	mapping := NewMapping(o.config.Matrix.Homeserver)
	mapping.MergeUsers(result.UserMapping)
	mapping.MergeTeams(result.SpaceMapping)
	mapping.MergeChannels(result.RoomMapping)

	// Save mapping
	mappingFile := GenerateMappingFilename(o.config.Data.MappingsDir)
	if err := SaveMapping(mapping, mappingFile); err != nil {
		o.state.FailStep(StepImportAssets, err)
		o.SaveState()
		return fmt.Errorf("failed to save mapping: %w", err)
	}

	// Link rooms to spaces
	if progress != nil {
		progress("linking", 0, len(assets.Channels), "")
	}
	_, err = importer.LinkRoomsToSpaces(assets.Channels, result.SpaceMapping, result.RoomMapping, importProgress)
	if err != nil {
		// Non-critical error, continue
	}

	// Complete step
	o.state.CompleteStep(StepImportAssets, mappingFile)
	return o.SaveState()
}

// ExportMemberships exports memberships from Mattermost
func (o *Orchestrator) ExportMemberships(progress ProgressCallback) error {
	if o.mmClient == nil {
		return fmt.Errorf("not connected to Mattermost")
	}

	// Check if we can run this step
	canRun, reason := o.state.CanRunStep(StepExportMemberships)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Start step
	o.state.StartStep(StepExportMemberships)
	if err := o.SaveState(); err != nil {
		return err
	}

	// Create exporter
	exporter := mattermost.NewExporter(o.mmClient)

	// Export callback
	var exportProgress mattermost.ExportProgressCallback
	if progress != nil {
		exportProgress = func(stage string, current, total int) {
			progress(stage, current, total, "")
			o.state.UpdateStepProgress(StepExportMemberships, current, total)
		}
	}

	// Export memberships
	memberships, err := exporter.ExportMemberships(exportProgress)
	if err != nil {
		o.state.FailStep(StepExportMemberships, err)
		o.SaveState()
		return fmt.Errorf("export failed: %w", err)
	}

	// Filter to active memberships
	memberships = mattermost.FilterActiveMemberships(memberships)

	// Generate filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("mattermost-memberships-%s.json.gz", timestamp)
	filepath := o.config.Data.AssetsDir + "/" + filename

	// Save to gzipped JSON
	if err := archive.SaveGzipJSON(filepath, memberships); err != nil {
		o.state.FailStep(StepExportMemberships, err)
		o.SaveState()
		return fmt.Errorf("failed to save memberships: %w", err)
	}

	// Complete step
	o.state.CompleteStep(StepExportMemberships, filepath)
	return o.SaveState()
}

// ImportMemberships imports memberships to Matrix
func (o *Orchestrator) ImportMemberships(progress ProgressCallback) error {
	if o.mxClient == nil {
		return fmt.Errorf("not connected to Matrix")
	}

	// Check if we can run this step
	canRun, reason := o.state.CanRunStep(StepImportMemberships)
	if !canRun {
		return fmt.Errorf("cannot run step: %s", reason)
	}

	// Get the membership file and mapping file from previous steps
	membershipFile := o.state.GetStepOutputFile(StepExportMemberships)
	if membershipFile == "" {
		return fmt.Errorf("no membership file found from export step")
	}

	mappingFile := o.state.GetStepOutputFile(StepImportAssets)
	if mappingFile == "" {
		return fmt.Errorf("no mapping file found from import assets step")
	}

	// Start step
	o.state.StartStep(StepImportMemberships)
	if err := o.SaveState(); err != nil {
		return err
	}

	// Load memberships
	var memberships mattermost.Memberships
	if err := archive.LoadGzipJSON(membershipFile, &memberships); err != nil {
		o.state.FailStep(StepImportMemberships, err)
		o.SaveState()
		return fmt.Errorf("failed to load memberships: %w", err)
	}

	// Load mapping
	mapping, err := LoadMapping(mappingFile)
	if err != nil {
		o.state.FailStep(StepImportMemberships, err)
		o.SaveState()
		return fmt.Errorf("failed to load mapping: %w", err)
	}

	// Create importer
	importer := matrix.NewImporter(o.mxClient)

	// Import callback
	var importProgress matrix.ImportProgressCallback
	if progress != nil {
		importProgress = func(stage string, current, total int, item string) {
			progress(stage, current, total, item)
			o.state.UpdateStepProgress(StepImportMemberships, current, total)
		}
	}

	// Apply team memberships
	if progress != nil {
		progress("team_memberships", 0, len(memberships.TeamMembers), "")
	}
	_, err = importer.ApplyTeamMemberships(memberships.TeamMembers, mapping.Users, mapping.Teams, importProgress)
	if err != nil {
		o.state.FailStep(StepImportMemberships, err)
		o.SaveState()
		return fmt.Errorf("failed to apply team memberships: %w", err)
	}

	// Apply channel memberships
	if progress != nil {
		progress("channel_memberships", 0, len(memberships.ChannelMembers), "")
	}
	_, err = importer.ApplyChannelMemberships(memberships.ChannelMembers, mapping.Users, mapping.Channels, importProgress)
	if err != nil {
		o.state.FailStep(StepImportMemberships, err)
		o.SaveState()
		return fmt.Errorf("failed to apply channel memberships: %w", err)
	}

	// Complete step
	o.state.CompleteStep(StepImportMemberships, "")
	return o.SaveState()
}

// TestMattermostConnection tests the Mattermost connection
func (o *Orchestrator) TestMattermostConnection() error {
	cfg := o.config.Mattermost
	passphrase := o.config.GetSSHKeyPassphrase("mattermost")
	sshPassword := o.config.GetSSHPassword("mattermost")

	// Test SSH connection first
	if err := ssh.TestConnectionWithPassword(cfg.SSH, passphrase, sshPassword); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	// If not using manual config, test reading config.json
	if !o.config.HasManualDatabaseConfig() {
		_, err := mattermost.GetDatabaseCredentials(cfg.SSH, passphrase, sshPassword, cfg.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read Mattermost config: %w", err)
		}
	}

	// Connect and test database
	if err := o.ConnectMattermost(); err != nil {
		return err
	}

	// Test database query
	if err := o.mmClient.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// TestMatrixConnection tests the Matrix connection
func (o *Orchestrator) TestMatrixConnection() error {
	cfg := o.config.Matrix
	passphrase := o.config.GetSSHKeyPassphrase("matrix")
	sshPassword := o.config.GetSSHPassword("matrix")

	// Test SSH connection first
	if err := ssh.TestConnectionWithPassword(cfg.SSH, passphrase, sshPassword); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	// Connect and test API
	if err := o.ConnectMatrix(); err != nil {
		return err
	}

	return nil
}
