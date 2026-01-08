package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MessageMapping represents the mapping between Mattermost posts and Matrix events
type MessageMapping struct {
	Version    string                     `json:"version"`
	CreatedAt  int64                      `json:"created_at"`
	UpdatedAt  int64                      `json:"updated_at"`
	Homeserver string                     `json:"homeserver"`
	Messages   map[string]*MessageMapEntry `json:"messages"` // key: Mattermost post ID
	mu         sync.RWMutex               `json:"-"`
}

// MessageMapEntry represents a single message mapping
type MessageMapEntry struct {
	MattermostID  string `json:"mattermost_id"`   // Mattermost post ID
	MatrixEventID string `json:"matrix_event_id"` // Matrix event ID ($xxx)
	ChannelID     string `json:"channel_id"`      // Mattermost channel ID
	RoomID        string `json:"room_id"`         // Matrix room ID
	UserID        string `json:"user_id"`         // Mattermost user ID
	MatrixUserID  string `json:"matrix_user_id"`  // Matrix user ID
	Timestamp     int64  `json:"timestamp"`       // Original message timestamp
	ImportedAt    int64  `json:"imported_at"`     // When this was imported
	IsReply       bool   `json:"is_reply"`        // Whether this is a reply
	RootID        string `json:"root_id,omitempty"` // Mattermost root post ID if reply
}

// NewMessageMapping creates a new message mapping
func NewMessageMapping(homeserver string) *MessageMapping {
	now := time.Now().UnixMilli()
	return &MessageMapping{
		Version:    "1.0",
		CreatedAt:  now,
		UpdatedAt:  now,
		Homeserver: homeserver,
		Messages:   make(map[string]*MessageMapEntry),
	}
}

// AddMessage adds a message mapping
func (m *MessageMapping) AddMessage(entry *MessageMapEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	entry.ImportedAt = time.Now().UnixMilli()
	m.Messages[entry.MattermostID] = entry
	m.UpdatedAt = time.Now().UnixMilli()
}

// GetMessage returns a message mapping by Mattermost ID
func (m *MessageMapping) GetMessage(mattermostID string) (*MessageMapEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.Messages[mattermostID]
	return entry, exists
}

// HasMessage checks if a message has already been imported
func (m *MessageMapping) HasMessage(mattermostID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.Messages[mattermostID]
	return exists
}

// GetMatrixEventID returns the Matrix event ID for a Mattermost post
func (m *MessageMapping) GetMatrixEventID(mattermostID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if entry, exists := m.Messages[mattermostID]; exists {
		return entry.MatrixEventID
	}
	return ""
}

// Count returns the number of mapped messages
func (m *MessageMapping) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Messages)
}

// GetStats returns statistics about the mapping
func (m *MessageMapping) GetStats() MessageMappingStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := MessageMappingStats{
		Total:        len(m.Messages),
		ByChannel:   make(map[string]int),
		ByRoom:      make(map[string]int),
	}
	
	for _, entry := range m.Messages {
		if entry.IsReply {
			stats.Replies++
		}
		stats.ByChannel[entry.ChannelID]++
		stats.ByRoom[entry.RoomID]++
	}
	
	return stats
}

// MessageMappingStats holds statistics about message mappings
type MessageMappingStats struct {
	Total     int            `json:"total"`
	Replies   int            `json:"replies"`
	ByChannel map[string]int `json:"by_channel"`
	ByRoom    map[string]int `json:"by_room"`
}

// SaveMessageMapping saves the message mapping to a file
func SaveMessageMapping(mapping *MessageMapping, filepath string) error {
	mapping.mu.RLock()
	defer mapping.mu.RUnlock()
	
	mapping.UpdatedAt = time.Now().UnixMilli()
	
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal message mapping: %w", err)
	}
	
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write message mapping file: %w", err)
	}
	
	return nil
}

// LoadMessageMapping loads a message mapping from a file
func LoadMessageMapping(filepath string) (*MessageMapping, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read message mapping file: %w", err)
	}
	
	var mapping MessageMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message mapping: %w", err)
	}
	
	if mapping.Messages == nil {
		mapping.Messages = make(map[string]*MessageMapEntry)
	}
	
	return &mapping, nil
}

// GenerateMessageMappingFilename generates a filename for message mapping
func GenerateMessageMappingFilename(dir string) string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("message-mapping-%s.json", timestamp))
}

// GetLatestMessageMappingFile finds the latest message mapping file in a directory
func GetLatestMessageMappingFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "message-mapping-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	
	if len(matches) == 0 {
		return "", nil
	}
	
	// Return the latest (last alphabetically due to timestamp format)
	latest := matches[0]
	for _, match := range matches[1:] {
		if match > latest {
			latest = match
		}
	}
	
	return latest, nil
}
