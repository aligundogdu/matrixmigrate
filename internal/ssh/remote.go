package ssh

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/aligundogdu/matrixmigrate/internal/config"
)

// RemoteExecutor executes commands on remote servers via SSH
type RemoteExecutor struct {
	client *ssh.Client
}

// NewRemoteExecutor creates a new remote executor with key auth
func NewRemoteExecutor(cfg config.SSHConfig, passphrase string) (*RemoteExecutor, error) {
	return NewRemoteExecutorWithPassword(cfg, passphrase, "")
}

// NewRemoteExecutorWithPassword creates a new remote executor with optional password auth
func NewRemoteExecutorWithPassword(cfg config.SSHConfig, passphrase, password string) (*RemoteExecutor, error) {
	// Build auth methods
	authMethods, err := buildAuthMethods(cfg, passphrase, password)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth methods: %w", err)
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	sshAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", sshAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	return &RemoteExecutor{client: client}, nil
}

// Close closes the SSH connection
func (r *RemoteExecutor) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// ReadFile reads a file from the remote server
func (r *RemoteExecutor) ReadFile(path string) ([]byte, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Use cat to read the file, with sudo if needed
	cmd := fmt.Sprintf("cat %s 2>/dev/null || sudo cat %s", path, path)
	if err := session.Run(cmd); err != nil {
		return nil, fmt.Errorf("failed to read file: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}

// FileExists checks if a file exists on the remote server
func (r *RemoteExecutor) FileExists(path string) (bool, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("test -f %s && echo 'exists'", path)
	output, err := session.Output(cmd)
	if err != nil {
		return false, nil // File doesn't exist
	}

	return bytes.Contains(output, []byte("exists")), nil
}

// ExecuteCommand executes a command on the remote server
func (r *RemoteExecutor) ExecuteCommand(cmd string) (string, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// GetClient returns the underlying SSH client (for creating tunnels)
func (r *RemoteExecutor) GetClient() *ssh.Client {
	return r.client
}
