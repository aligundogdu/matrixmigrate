package version

import (
	"fmt"
	"runtime"
)

// These variables are set at build time using ldflags
var (
	// Version is the semantic version (e.g., "1.0.0")
	Version = "dev"
	
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	
	// BuildTime is the build timestamp
	BuildTime = "unknown"
	
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version with commit hash
func GetFullVersion() string {
	if GitCommit == "unknown" || GitCommit == "" {
		return Version
	}
	// Show first 7 characters of commit hash
	commit := GitCommit
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return fmt.Sprintf("%s (%s)", Version, commit)
}

// GetBuildInfo returns detailed build information
func GetBuildInfo() string {
	return fmt.Sprintf("Version:    %s\nGit Commit: %s\nBuild Time: %s\nGo Version: %s\nOS/Arch:    %s/%s",
		Version,
		GitCommit,
		BuildTime,
		GoVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
}

// GetShortInfo returns a one-line version info
func GetShortInfo() string {
	return fmt.Sprintf("MatrixMigrate %s", GetFullVersion())
}
