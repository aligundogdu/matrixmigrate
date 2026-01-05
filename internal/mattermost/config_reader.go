package mattermost

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aligundogdu/matrixmigrate/internal/config"
	"github.com/aligundogdu/matrixmigrate/internal/ssh"
)

// MattermostConfig represents the Mattermost config.json structure
type MattermostConfig struct {
	SqlSettings SqlSettings `json:"SqlSettings"`
}

// SqlSettings represents the SQL settings in Mattermost config
type SqlSettings struct {
	DriverName string `json:"DriverName"`
	DataSource string `json:"DataSource"`
}

// DatabaseCredentials holds parsed database credentials
type DatabaseCredentials struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
}

// DefaultConfigPaths are the common locations for Mattermost config
var DefaultConfigPaths = []string{
	"/opt/mattermost/config/config.json",
	"/opt/mattermost/config.json",
	"/etc/mattermost/config.json",
	"./config/config.json",
}

// ReadConfigFromRemote reads Mattermost config.json from remote server via SSH
func ReadConfigFromRemote(sshCfg config.SSHConfig, passphrase, password string, configPath string) (*MattermostConfig, error) {
	// Create SSH executor
	executor, err := ssh.NewRemoteExecutorWithPassword(sshCfg, passphrase, password)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer executor.Close()

	// If config path not specified, try default locations
	paths := []string{configPath}
	if configPath == "" {
		paths = DefaultConfigPaths
	}

	var configData []byte
	var foundPath string

	for _, path := range paths {
		if path == "" {
			continue
		}
		exists, err := executor.FileExists(path)
		if err != nil {
			continue
		}
		if exists {
			data, err := executor.ReadFile(path)
			if err != nil {
				continue
			}
			configData = data
			foundPath = path
			break
		}
	}

	if configData == nil {
		return nil, fmt.Errorf("could not find Mattermost config.json in any of the default locations")
	}

	// Parse JSON
	var mmConfig MattermostConfig
	if err := json.Unmarshal(configData, &mmConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config.json from %s: %w", foundPath, err)
	}

	return &mmConfig, nil
}

// ParseDataSource parses the PostgreSQL connection string from Mattermost config
func ParseDataSource(dataSource string) (*DatabaseCredentials, error) {
	creds := &DatabaseCredentials{
		Port:    5432,
		SSLMode: "disable",
	}

	// Mattermost uses format: postgres://user:password@host:port/database?sslmode=disable
	// or: user:password@host:port/database?sslmode=disable

	// Try parsing as URL
	if strings.HasPrefix(dataSource, "postgres://") || strings.HasPrefix(dataSource, "postgresql://") {
		u, err := url.Parse(dataSource)
		if err != nil {
			return nil, fmt.Errorf("failed to parse data source URL: %w", err)
		}

		creds.Host = u.Hostname()
		if port := u.Port(); port != "" {
			fmt.Sscanf(port, "%d", &creds.Port)
		}
		creds.User = u.User.Username()
		creds.Password, _ = u.User.Password()
		creds.Database = strings.TrimPrefix(u.Path, "/")

		// Parse query params
		if sslmode := u.Query().Get("sslmode"); sslmode != "" {
			creds.SSLMode = sslmode
		}
	} else {
		// Try parsing as key=value format: host=localhost port=5432 user=mmuser password=xxx dbname=mattermost
		parts := strings.Fields(dataSource)
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key, value := kv[0], kv[1]
			switch key {
			case "host":
				creds.Host = value
			case "port":
				fmt.Sscanf(value, "%d", &creds.Port)
			case "user":
				creds.User = value
			case "password":
				creds.Password = value
			case "dbname":
				creds.Database = value
			case "sslmode":
				creds.SSLMode = value
			}
		}
	}

	// Validate
	if creds.Host == "" {
		return nil, fmt.Errorf("could not parse host from data source")
	}
	if creds.Database == "" {
		return nil, fmt.Errorf("could not parse database name from data source")
	}
	if creds.User == "" {
		return nil, fmt.Errorf("could not parse user from data source")
	}

	return creds, nil
}

// GetDatabaseCredentials reads Mattermost config and returns database credentials
func GetDatabaseCredentials(sshCfg config.SSHConfig, passphrase, password string, configPath string) (*DatabaseCredentials, error) {
	// Read config from remote
	mmConfig, err := ReadConfigFromRemote(sshCfg, passphrase, password, configPath)
	if err != nil {
		return nil, err
	}

	// Check driver
	if mmConfig.SqlSettings.DriverName != "postgres" {
		return nil, fmt.Errorf("unsupported database driver: %s (only postgres is supported)", mmConfig.SqlSettings.DriverName)
	}

	// Parse data source
	creds, err := ParseDataSource(mmConfig.SqlSettings.DataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data source: %w", err)
	}

	return creds, nil
}
