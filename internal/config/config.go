package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the main configuration structure
type Config struct {
	Language   string           `mapstructure:"language"`
	Mattermost MattermostConfig `mapstructure:"mattermost"`
	Matrix     MatrixConfig     `mapstructure:"matrix"`
	Data       DataConfig       `mapstructure:"data"`
}

// MattermostConfig holds Mattermost server configuration
type MattermostConfig struct {
	SSH        SSHConfig      `mapstructure:"ssh"`
	ConfigPath string         `mapstructure:"config_path"` // Path to config.json on remote server
	Database   DatabaseConfig `mapstructure:"database"`    // Optional: manual override
}

// MatrixConfig holds Matrix server configuration
type MatrixConfig struct {
	SSH        SSHConfig   `mapstructure:"ssh"`
	API        APIConfig   `mapstructure:"api"`
	Auth       AuthConfig  `mapstructure:"auth"` // Username/password auth for Matrix API
	Homeserver string      `mapstructure:"homeserver"`
}

// SSHConfig holds SSH connection configuration
type SSHConfig struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	User          string `mapstructure:"user"`
	KeyPath       string `mapstructure:"key_path"`       // Optional: path to SSH key
	PassphraseEnv string `mapstructure:"passphrase_env"` // Optional: env var for key passphrase
	PasswordEnv   string `mapstructure:"password_env"`   // Optional: env var for SSH password
}

// DatabaseConfig holds PostgreSQL connection configuration (optional manual override)
type DatabaseConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Name        string `mapstructure:"name"`
	User        string `mapstructure:"user"`
	PasswordEnv string `mapstructure:"password_env"`
}

// APIConfig holds Matrix API configuration
type APIConfig struct {
	BaseURL       string `mapstructure:"base_url"`
	AdminTokenEnv string `mapstructure:"admin_token_env"` // Optional: if provided, use this token
}

// AuthConfig holds Matrix authentication configuration
type AuthConfig struct {
	Username    string `mapstructure:"username"`     // Admin username
	PasswordEnv string `mapstructure:"password_env"` // Env var for password
}

// DataConfig holds data storage paths
type DataConfig struct {
	AssetsDir   string `mapstructure:"assets_dir"`
	MappingsDir string `mapstructure:"mappings_dir"`
	StateFile   string `mapstructure:"state_file"`
}

// Load loads configuration from the specified file or default locations
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory and home directory
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.matrixmigrate")
	}

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			return loadDefaults(v)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths
	cfg.expandPaths()

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	v.SetDefault("language", "en")
	v.SetDefault("mattermost.ssh.port", 22)
	v.SetDefault("mattermost.config_path", "/opt/mattermost/config/config.json")
	v.SetDefault("mattermost.database.host", "localhost")
	v.SetDefault("mattermost.database.port", 5432)
	v.SetDefault("matrix.ssh.port", 22)
	v.SetDefault("matrix.api.base_url", "http://localhost:8008")
	v.SetDefault("data.assets_dir", "./data/assets")
	v.SetDefault("data.mappings_dir", "./data/mappings")
	v.SetDefault("data.state_file", "./data/state.json")
}

// loadDefaults creates a config with default values
func loadDefaults(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal defaults: %w", err)
	}
	cfg.expandPaths()
	return &cfg, nil
}

// expandPaths expands ~ and environment variables in paths
func (c *Config) expandPaths() {
	c.Mattermost.SSH.KeyPath = expandPath(c.Mattermost.SSH.KeyPath)
	c.Matrix.SSH.KeyPath = expandPath(c.Matrix.SSH.KeyPath)
	c.Data.AssetsDir = expandPath(c.Data.AssetsDir)
	c.Data.MappingsDir = expandPath(c.Data.MappingsDir)
	c.Data.StateFile = expandPath(c.Data.StateFile)
}

// expandPath expands ~ to home directory and resolves environment variables
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	return path
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Mattermost config if SSH host is provided
	if c.Mattermost.SSH.Host != "" {
		if c.Mattermost.SSH.User == "" {
			return fmt.Errorf("mattermost.ssh.user is required")
		}
		// Either key_path or password_env must be provided
		hasKey := c.Mattermost.SSH.KeyPath != ""
		hasPassword := c.Mattermost.SSH.PasswordEnv != ""
		if !hasKey && !hasPassword {
			return fmt.Errorf("mattermost.ssh: either key_path or password_env is required")
		}
	}

	// Validate Matrix config if SSH host is provided
	if c.Matrix.SSH.Host != "" {
		if c.Matrix.SSH.User == "" {
			return fmt.Errorf("matrix.ssh.user is required")
		}
		// Either key_path or password_env must be provided
		hasKey := c.Matrix.SSH.KeyPath != ""
		hasPassword := c.Matrix.SSH.PasswordEnv != ""
		if !hasKey && !hasPassword {
			return fmt.Errorf("matrix.ssh: either key_path or password_env is required")
		}
		if c.Matrix.Homeserver == "" {
			return fmt.Errorf("matrix.homeserver is required")
		}
		// Check that either auth or admin token is provided
		hasAuth := c.Matrix.Auth.Username != "" && c.Matrix.Auth.PasswordEnv != ""
		hasToken := c.Matrix.API.AdminTokenEnv != ""
		if !hasAuth && !hasToken {
			return fmt.Errorf("matrix: either auth (username/password_env) or api.admin_token_env is required")
		}
	}

	return nil
}

// HasManualDatabaseConfig returns true if database config is manually specified
func (c *Config) HasManualDatabaseConfig() bool {
	return c.Mattermost.Database.Host != "" && 
		c.Mattermost.Database.Name != "" && 
		c.Mattermost.Database.User != ""
}

// GetMattermostDBPassword returns the Mattermost database password from environment
func (c *Config) GetMattermostDBPassword() string {
	if c.Mattermost.Database.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.Mattermost.Database.PasswordEnv)
}

// GetMatrixAdminToken returns the Matrix admin token from environment
func (c *Config) GetMatrixAdminToken() string {
	if c.Matrix.API.AdminTokenEnv == "" {
		return ""
	}
	return os.Getenv(c.Matrix.API.AdminTokenEnv)
}

// GetMatrixPassword returns the Matrix password from environment
func (c *Config) GetMatrixPassword() string {
	if c.Matrix.Auth.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.Matrix.Auth.PasswordEnv)
}

// UseTokenAuth returns true if admin token should be used instead of login
func (c *Config) UseTokenAuth() bool {
	return c.GetMatrixAdminToken() != ""
}

// GetSSHKeyPassphrase returns the SSH key passphrase from environment
func (c *Config) GetSSHKeyPassphrase(server string) string {
	var envVar string
	switch server {
	case "mattermost":
		envVar = c.Mattermost.SSH.PassphraseEnv
	case "matrix":
		envVar = c.Matrix.SSH.PassphraseEnv
	}
	if envVar == "" {
		return ""
	}
	return os.Getenv(envVar)
}

// GetSSHPassword returns the SSH password from environment
func (c *Config) GetSSHPassword(server string) string {
	var envVar string
	switch server {
	case "mattermost":
		envVar = c.Mattermost.SSH.PasswordEnv
	case "matrix":
		envVar = c.Matrix.SSH.PasswordEnv
	}
	if envVar == "" {
		return ""
	}
	return os.Getenv(envVar)
}

// EnsureDataDirs creates data directories if they don't exist
func (c *Config) EnsureDataDirs() error {
	dirs := []string{c.Data.AssetsDir, c.Data.MappingsDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Ensure state file directory exists
	stateDir := filepath.Dir(c.Data.StateFile)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory %s: %w", stateDir, err)
	}

	return nil
}

// MattermostDSN returns the PostgreSQL connection string for Mattermost
func (c *Config) MattermostDSN() string {
	password := c.GetMattermostDBPassword()
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Mattermost.Database.Host,
		c.Mattermost.Database.Port,
		c.Mattermost.Database.User,
		password,
		c.Mattermost.Database.Name,
	)
}

// MatrixAPIURL returns the full Matrix API base URL
func (c *Config) MatrixAPIURL() string {
	return strings.TrimSuffix(c.Matrix.API.BaseURL, "/")
}

// FormatUserID formats a username as a Matrix user ID
func (c *Config) FormatUserID(username string) string {
	return fmt.Sprintf("@%s:%s", username, c.Matrix.Homeserver)
}
