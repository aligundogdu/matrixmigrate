package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Mapping represents the ID mappings between Mattermost and Matrix
type Mapping struct {
	Version     string            `json:"version"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
	Homeserver  string            `json:"homeserver"`
	Users       map[string]string `json:"users"`       // mm_user_id -> matrix_user_id
	Teams       map[string]string `json:"teams"`       // mm_team_id -> matrix_space_id
	Channels    map[string]string `json:"channels"`    // mm_channel_id -> matrix_room_id
}

// NewMapping creates a new empty mapping
func NewMapping(homeserver string) *Mapping {
	now := time.Now().UnixMilli()
	return &Mapping{
		Version:    "1.0",
		CreatedAt:  now,
		UpdatedAt:  now,
		Homeserver: homeserver,
		Users:      make(map[string]string),
		Teams:      make(map[string]string),
		Channels:   make(map[string]string),
	}
}

// MergeUsers merges user mappings
func (m *Mapping) MergeUsers(users map[string]string) {
	for k, v := range users {
		m.Users[k] = v
	}
	m.UpdatedAt = time.Now().UnixMilli()
}

// MergeTeams merges team mappings
func (m *Mapping) MergeTeams(teams map[string]string) {
	for k, v := range teams {
		m.Teams[k] = v
	}
	m.UpdatedAt = time.Now().UnixMilli()
}

// MergeChannels merges channel mappings
func (m *Mapping) MergeChannels(channels map[string]string) {
	for k, v := range channels {
		m.Channels[k] = v
	}
	m.UpdatedAt = time.Now().UnixMilli()
}

// GetMatrixUserID returns the Matrix user ID for a Mattermost user ID
func (m *Mapping) GetMatrixUserID(mmUserID string) (string, bool) {
	id, ok := m.Users[mmUserID]
	return id, ok
}

// GetMatrixSpaceID returns the Matrix space ID for a Mattermost team ID
func (m *Mapping) GetMatrixSpaceID(mmTeamID string) (string, bool) {
	id, ok := m.Teams[mmTeamID]
	return id, ok
}

// GetMatrixRoomID returns the Matrix room ID for a Mattermost channel ID
func (m *Mapping) GetMatrixRoomID(mmChannelID string) (string, bool) {
	id, ok := m.Channels[mmChannelID]
	return id, ok
}

// Stats returns statistics about the mapping
func (m *Mapping) Stats() MappingStats {
	return MappingStats{
		UsersCount:    len(m.Users),
		TeamsCount:    len(m.Teams),
		ChannelsCount: len(m.Channels),
	}
}

// MappingStats holds mapping statistics
type MappingStats struct {
	UsersCount    int `json:"users_count"`
	TeamsCount    int `json:"teams_count"`
	ChannelsCount int `json:"channels_count"`
}

// SaveMapping saves a mapping to a JSON file
func SaveMapping(mapping *Mapping, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write mapping file: %w", err)
	}

	return nil
}

// LoadMapping loads a mapping from a JSON file
func LoadMapping(filePath string) (*Mapping, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mapping file: %w", err)
	}

	var mapping Mapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to parse mapping file: %w", err)
	}

	return &mapping, nil
}

// MappingExists checks if a mapping file exists
func MappingExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// GetLatestMappingFile finds the most recent mapping file in a directory
func GetLatestMappingFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "asset-mapping-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob mapping files: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no mapping files found")
	}

	// Find the most recent file
	var latest string
	var latestTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if latest == "" || info.ModTime().After(latestTime) {
			latest = match
			latestTime = info.ModTime()
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no valid mapping files found")
	}

	return latest, nil
}

// GenerateMappingFilename generates a filename for a new mapping file
func GenerateMappingFilename(dir string) string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("asset-mapping-%s.json", timestamp))
}




