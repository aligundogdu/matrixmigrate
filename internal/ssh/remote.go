package ssh

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/aligundogdu/matrixmigrate/internal/config"
)

// RemoteExecutor executes commands on remote servers via SSH
type RemoteExecutor struct {
	client *ssh.Client
	// sudoPassword, when non-empty, is sent on stdin for sudo -S when reading files (see ReadFile).
	sudoPassword string
}

// SetFileReadSudoPassword enables sudo -S for ReadFile. The password is the remote account's
// sudo password (not the SSH key passphrase). Prefer granting the SSH user read ACL instead.
func (r *RemoteExecutor) SetFileReadSudoPassword(password string) {
	r.sudoPassword = strings.TrimSpace(password)
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

// shellSingleQuoted escapes a path for safe use inside single quotes in a remote shell.
func shellSingleQuoted(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
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
	q := shellSingleQuoted(path)

	if r.sudoPassword != "" {
		return r.readFileWithSudoStdin(q)
	}

	session, err := r.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Try unprivileged read first, then sudo -n only. Plain `sudo` over SSH is non-interactive
	// and fails with "a terminal is required to read the password" when sudo would prompt.
	cmd := fmt.Sprintf("cat %s 2>/dev/null || sudo -n cat %s", q, q)
	if err := session.Run(cmd); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(msg, "a password is required") || strings.Contains(msg, "terminal is required") {
			msg += " — fix: grant the SSH user read access to the Mattermost data directory (e.g. group/ACL), configure NOPASSWD, or set mattermost.ssh.file_read_sudo_password_env for sudo -S (see security note in config)."
		}
		return nil, fmt.Errorf("failed to read file: %s", msg)
	}

	return stdout.Bytes(), nil
}

// readFileWithSudoStdin runs: cat path || sudo -S cat path with password on stdin (non-interactive).
func (r *RemoteExecutor) readFileWithSudoStdin(q string) ([]byte, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	session.Stdin = bytes.NewReader([]byte(r.sudoPassword + "\n"))

	cmd := fmt.Sprintf("bash -c \"cat %s 2>/dev/null || sudo -S -p '' cat %s\"", q, q)
	if err := session.Run(cmd); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("failed to read file (sudo -S): %s", msg)
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

	cmd := fmt.Sprintf("test -f %s && echo 'exists'", shellSingleQuoted(path))
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
