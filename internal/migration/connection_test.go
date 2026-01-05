package migration

import (
	"fmt"
	"os"

	"github.com/aligundogdu/matrixmigrate/internal/config"
	"github.com/aligundogdu/matrixmigrate/internal/mattermost"
	"github.com/aligundogdu/matrixmigrate/internal/matrix"
	"github.com/aligundogdu/matrixmigrate/internal/ssh"
)

// TestStep represents a single test step
type TestStep struct {
	Name        string
	Description string
	Status      TestStatus
	Error       string
	Details     string
}

// TestStatus represents the status of a test step
type TestStatus string

const (
	TestPending  TestStatus = "pending"
	TestRunning  TestStatus = "running"
	TestPassed   TestStatus = "passed"
	TestFailed   TestStatus = "failed"
	TestSkipped  TestStatus = "skipped"
	TestWarning  TestStatus = "warning"
)

// ConnectionTestResult holds all test results
type ConnectionTestResult struct {
	ConfigSteps     []TestStep
	MattermostSteps []TestStep
	MatrixSteps     []TestStep
	AllPassed       bool
}

// TestCallback is called for each test step
type TestCallback func(server string, step *TestStep)

// RunConnectionTests runs all connection tests with detailed steps
func RunConnectionTests(cfg *config.Config, callback TestCallback) *ConnectionTestResult {
	result := &ConnectionTestResult{
		AllPassed: true,
	}

	// Config tests
	result.ConfigSteps = runConfigTests(cfg, callback)
	for _, step := range result.ConfigSteps {
		if step.Status == TestFailed {
			result.AllPassed = false
		}
	}

	// Mattermost tests
	result.MattermostSteps = runMattermostTests(cfg, callback)
	for _, step := range result.MattermostSteps {
		if step.Status == TestFailed {
			result.AllPassed = false
		}
	}

	// Matrix tests
	result.MatrixSteps = runMatrixTests(cfg, callback)
	for _, step := range result.MatrixSteps {
		if step.Status == TestFailed {
			result.AllPassed = false
		}
	}

	return result
}

// runConfigTests runs configuration validation tests
func runConfigTests(cfg *config.Config, callback TestCallback) []TestStep {
	steps := []TestStep{}

	// Step 1: Config file exists
	step := TestStep{
		Name:        "config_file",
		Description: "Configuration file loaded",
		Status:      TestPassed,
		Details:     "config.yaml found and parsed",
	}
	if cfg == nil {
		step.Status = TestFailed
		step.Error = "Configuration file not found or invalid"
	}
	if callback != nil {
		callback("config", &step)
	}
	steps = append(steps, step)

	if cfg == nil {
		return steps
	}

	// Step 2: Data directories
	step = TestStep{
		Name:        "data_dirs",
		Description: "Data directories accessible",
		Status:      TestPassed,
		Details:     fmt.Sprintf("Assets: %s, Mappings: %s", cfg.Data.AssetsDir, cfg.Data.MappingsDir),
	}
	if err := cfg.EnsureDataDirs(); err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
	}
	if callback != nil {
		callback("config", &step)
	}
	steps = append(steps, step)

	return steps
}

// runMattermostTests runs Mattermost connection tests
func runMattermostTests(cfg *config.Config, callback TestCallback) []TestStep {
	steps := []TestStep{}

	if cfg == nil {
		return steps
	}

	// Step 1: SSH configuration
	step := TestStep{
		Name:        "mm_ssh_config",
		Description: "SSH configuration",
		Status:      TestPending,
	}
	
	if cfg.Mattermost.SSH.Host == "" {
		step.Status = TestSkipped
		step.Details = "SSH host not configured"
		if callback != nil {
			callback("mattermost", &step)
		}
		steps = append(steps, step)
		return steps
	}

	// Check auth method
	hasKey := cfg.Mattermost.SSH.KeyPath != ""
	hasPassword := cfg.Mattermost.SSH.PasswordEnv != ""
	
	if hasKey {
		// Check if key file exists
		if _, err := os.Stat(cfg.Mattermost.SSH.KeyPath); err != nil {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("SSH key not found: %s", cfg.Mattermost.SSH.KeyPath)
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Key: %s", cfg.Mattermost.SSH.KeyPath)
		}
	} else if hasPassword {
		password := cfg.GetSSHPassword("mattermost")
		if password == "" {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("SSH password env var not set: %s", cfg.Mattermost.SSH.PasswordEnv)
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Password auth via $%s", cfg.Mattermost.SSH.PasswordEnv)
		}
	} else {
		step.Status = TestFailed
		step.Error = "No SSH authentication method configured"
	}
	
	if callback != nil {
		callback("mattermost", &step)
	}
	steps = append(steps, step)

	if step.Status == TestFailed {
		return steps
	}

	// Step 2: SSH connection
	step = TestStep{
		Name:        "mm_ssh_connect",
		Description: "SSH connection",
		Status:      TestRunning,
		Details:     fmt.Sprintf("%s@%s:%d", cfg.Mattermost.SSH.User, cfg.Mattermost.SSH.Host, cfg.Mattermost.SSH.Port),
	}
	if callback != nil {
		callback("mattermost", &step)
	}

	passphrase := cfg.GetSSHKeyPassphrase("mattermost")
	sshPassword := cfg.GetSSHPassword("mattermost")
	
	err := ssh.TestConnectionWithPassword(cfg.Mattermost.SSH, passphrase, sshPassword)
	if err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
	} else {
		step.Status = TestPassed
	}
	if callback != nil {
		callback("mattermost", &step)
	}
	steps = append(steps, step)

	if step.Status == TestFailed {
		return steps
	}

	// Step 3: Config file read (if not manual DB config)
	if !cfg.HasManualDatabaseConfig() {
		step = TestStep{
			Name:        "mm_config_read",
			Description: "Mattermost config.json",
			Status:      TestRunning,
			Details:     cfg.Mattermost.ConfigPath,
		}
		if callback != nil {
			callback("mattermost", &step)
		}

		creds, err := mattermost.GetDatabaseCredentials(cfg.Mattermost.SSH, passphrase, sshPassword, cfg.Mattermost.ConfigPath)
		if err != nil {
			step.Status = TestFailed
			step.Error = err.Error()
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("DB: %s@%s:%d/%s", creds.User, creds.Host, creds.Port, creds.Database)
		}
		if callback != nil {
			callback("mattermost", &step)
		}
		steps = append(steps, step)

		if step.Status == TestFailed {
			return steps
		}
	}

	// Step 4: Database connection
	step = TestStep{
		Name:        "mm_db_connect",
		Description: "Database connection",
		Status:      TestRunning,
	}
	if callback != nil {
		callback("mattermost", &step)
	}

	// Create orchestrator temporarily to test DB
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
	} else {
		defer orch.Close()
		
		if err := orch.ConnectMattermost(); err != nil {
			step.Status = TestFailed
			step.Error = err.Error()
		} else {
			// Test query
			if err := orch.mmClient.Ping(); err != nil {
				step.Status = TestFailed
				step.Error = fmt.Sprintf("Database ping failed: %s", err.Error())
			} else {
				// Get some stats
				users, teams, channels, _ := mattermost.NewExporter(orch.mmClient).GetCounts()
				step.Status = TestPassed
				step.Details = fmt.Sprintf("%d users, %d teams, %d channels", users, teams, channels)
			}
		}
	}
	if callback != nil {
		callback("mattermost", &step)
	}
	steps = append(steps, step)

	return steps
}

// runMatrixTests runs Matrix connection tests
func runMatrixTests(cfg *config.Config, callback TestCallback) []TestStep {
	steps := []TestStep{}

	if cfg == nil {
		return steps
	}

	// Step 1: SSH configuration
	step := TestStep{
		Name:        "mx_ssh_config",
		Description: "SSH configuration",
		Status:      TestPending,
	}
	
	if cfg.Matrix.SSH.Host == "" {
		step.Status = TestSkipped
		step.Details = "SSH host not configured"
		if callback != nil {
			callback("matrix", &step)
		}
		steps = append(steps, step)
		return steps
	}

	// Check auth method
	hasKey := cfg.Matrix.SSH.KeyPath != ""
	hasPassword := cfg.Matrix.SSH.PasswordEnv != ""
	
	if hasKey {
		if _, err := os.Stat(cfg.Matrix.SSH.KeyPath); err != nil {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("SSH key not found: %s", cfg.Matrix.SSH.KeyPath)
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Key: %s", cfg.Matrix.SSH.KeyPath)
		}
	} else if hasPassword {
		password := cfg.GetSSHPassword("matrix")
		if password == "" {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("SSH password env var not set: %s", cfg.Matrix.SSH.PasswordEnv)
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Password auth via $%s", cfg.Matrix.SSH.PasswordEnv)
		}
	} else {
		step.Status = TestFailed
		step.Error = "No SSH authentication method configured"
	}
	
	if callback != nil {
		callback("matrix", &step)
	}
	steps = append(steps, step)

	if step.Status == TestFailed {
		return steps
	}

	// Step 2: SSH connection
	step = TestStep{
		Name:        "mx_ssh_connect",
		Description: "SSH connection",
		Status:      TestRunning,
		Details:     fmt.Sprintf("%s@%s:%d", cfg.Matrix.SSH.User, cfg.Matrix.SSH.Host, cfg.Matrix.SSH.Port),
	}
	if callback != nil {
		callback("matrix", &step)
	}

	passphrase := cfg.GetSSHKeyPassphrase("matrix")
	sshPassword := cfg.GetSSHPassword("matrix")
	
	err := ssh.TestConnectionWithPassword(cfg.Matrix.SSH, passphrase, sshPassword)
	if err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
	} else {
		step.Status = TestPassed
	}
	if callback != nil {
		callback("matrix", &step)
	}
	steps = append(steps, step)

	if step.Status == TestFailed {
		return steps
	}

	// Step 3: API authentication configuration
	step = TestStep{
		Name:        "mx_auth_config",
		Description: "API authentication",
		Status:      TestPending,
	}
	
	if cfg.UseTokenAuth() {
		token := cfg.GetMatrixAdminToken()
		if token == "" {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("Admin token env var not set: %s", cfg.Matrix.API.AdminTokenEnv)
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Token auth via $%s", cfg.Matrix.API.AdminTokenEnv)
		}
	} else {
		password := cfg.GetMatrixPassword()
		if password == "" {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("Matrix password env var not set: %s", cfg.Matrix.Auth.PasswordEnv)
		} else if cfg.Matrix.Auth.Username == "" {
			step.Status = TestFailed
			step.Error = "Matrix username not configured"
		} else {
			step.Status = TestPassed
			step.Details = fmt.Sprintf("Login as %s via $%s", cfg.Matrix.Auth.Username, cfg.Matrix.Auth.PasswordEnv)
		}
	}
	if callback != nil {
		callback("matrix", &step)
	}
	steps = append(steps, step)

	if step.Status == TestFailed {
		return steps
	}

	// Step 4: API connection and authentication
	step = TestStep{
		Name:        "mx_api_connect",
		Description: "API connection",
		Status:      TestRunning,
		Details:     fmt.Sprintf("Homeserver: %s", cfg.Matrix.Homeserver),
	}
	if callback != nil {
		callback("matrix", &step)
	}

	// Get local port for tunnel
	localPort, err := ssh.GetLocalPort()
	if err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
		if callback != nil {
			callback("matrix", &step)
		}
		steps = append(steps, step)
		return steps
	}

	// Create tunnel
	tunnelCfg := ssh.TunnelConfig{
		SSHConfig:  cfg.Matrix.SSH,
		LocalPort:  localPort,
		RemoteHost: "127.0.0.1",
		RemotePort: 8008,
		Passphrase: passphrase,
		Password:   sshPassword,
	}

	tunnel, err := ssh.NewTunnel(tunnelCfg)
	if err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
		if callback != nil {
			callback("matrix", &step)
		}
		steps = append(steps, step)
		return steps
	}
	defer tunnel.Close()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	// Get access token
	var accessToken string
	if cfg.UseTokenAuth() {
		accessToken = cfg.GetMatrixAdminToken()
	} else {
		loginResp, err := matrix.Login(baseURL, cfg.Matrix.Auth.Username, cfg.GetMatrixPassword())
		if err != nil {
			step.Status = TestFailed
			step.Error = fmt.Sprintf("Login failed: %s", err.Error())
			if callback != nil {
				callback("matrix", &step)
			}
			steps = append(steps, step)
			return steps
		}
		accessToken = loginResp.AccessToken
		step.Details = fmt.Sprintf("Logged in as %s", loginResp.UserID)
	}

	// Test API
	client := matrix.NewClient(baseURL, accessToken, cfg.Matrix.Homeserver)
	if err := client.TestConnection(); err != nil {
		step.Status = TestFailed
		step.Error = err.Error()
	} else {
		step.Status = TestPassed
	}
	if callback != nil {
		callback("matrix", &step)
	}
	steps = append(steps, step)

	return steps
}

// GetTestStatusIcon returns an icon for the test status
func GetTestStatusIcon(status TestStatus) string {
	switch status {
	case TestPending:
		return "○"
	case TestRunning:
		return "◐"
	case TestPassed:
		return "✓"
	case TestFailed:
		return "✗"
	case TestSkipped:
		return "⊘"
	case TestWarning:
		return "⚠"
	default:
		return "?"
	}
}


