package matrix

import (
	"fmt"
	"strings"

	"github.com/aligundogdu/matrixmigrate/internal/logger"
	"github.com/aligundogdu/matrixmigrate/internal/mattermost"
)

// Importer handles importing data to Matrix
type Importer struct {
	client *Client
}

// NewImporter creates a new importer
func NewImporter(client *Client) *Importer {
	return &Importer{client: client}
}

// ImportProgressCallback is called to report import progress
type ImportProgressCallback func(stage string, current, total int, item string)

// GenerateRandomPassword generates a random password for new users
func GenerateRandomPassword() string {
	// In production, use crypto/rand for secure random password
	return "ChangeMe123!" // Placeholder - users should change this
}

// ImportUsers imports users from Mattermost to Matrix
func (i *Importer) ImportUsers(users []mattermost.User, existingMapping map[string]string, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(users)

	logger.Info("Starting user import: %d users to process", total)

	// Copy existing mappings
	for k, v := range existingMapping {
		mapping[k] = v
	}
	logger.Info("Existing mappings copied: %d entries", len(existingMapping))

	for idx, user := range users {
		logger.Info("Processing user %d/%d: %s (ID: %s)", idx+1, total, user.Username, user.ID)
		
		if progress != nil {
			progress("users", idx+1, total, user.Username)
		}

		// Skip deleted users
		if user.IsDeleted() {
			logger.Info("User '%s' is deleted, skipping", user.Username)
			stats.UsersSkipped++
			continue
		}

		// Skip if already in mapping
		if _, exists := existingMapping[user.ID]; exists {
			logger.Info("User '%s' already in mapping, skipping", user.Username)
			stats.UsersSkipped++
			continue
		}

		// Try to check if user exists, but don't fail if check fails
		// (some Matrix servers only allow checking local users)
		exists := false
		existsCheck, err := i.client.UserExists(user.Username)
		if err != nil {
			// If check fails with "Can only look up local users", ignore it
			// CreateUser is idempotent anyway, so we can just try to create
			if strings.Contains(err.Error(), "Can only look up local users") {
				logger.Info("UserExists check not available for '%s', will try to create", user.Username)
			} else {
				logger.Warn("UserExists check failed for '%s': %v, will try to create anyway", user.Username, err)
			}
		} else {
			exists = existsCheck
		}

		if exists {
			// User already exists, just add to mapping
			mapping[user.ID] = i.client.FormatUserID(user.Username)
			logger.Info("User '%s' already exists, skipped", user.Username)
			stats.UsersSkipped++
			continue
		}

		// Create the user (CreateUser is idempotent - if user exists, it will update)
		displayName := strings.TrimSpace(user.FirstName + " " + user.LastName)
		if displayName == "" {
			displayName = user.Username
		}

		req := &CreateUserRequest{
			Password:    GenerateRandomPassword(),
			DisplayName: displayName,
			Admin:       false,
			Deactivated: false,
		}

		resp, err := i.client.CreateUser(user.Username, req)
		if err != nil {
			// Check if error is because user already exists
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "M_USER_IN_USE") {
				// User exists, add to mapping
				mapping[user.ID] = i.client.FormatUserID(user.Username)
				logger.Info("User '%s' already exists (detected during create), skipped", user.Username)
				stats.UsersSkipped++
				continue
			}
			logger.Error("Failed to create user '%s': %v", user.Username, err)
			stats.UsersFailed++
			continue
		}
		logger.Success("Created user '%s' -> %s", user.Username, resp.UserID)

		mapping[user.ID] = resp.UserID
		stats.UsersCreated++
	}

	return mapping, stats, nil
}

// ImportTeamsAsSpaces imports teams from Mattermost as Matrix spaces
func (i *Importer) ImportTeamsAsSpaces(teams []mattermost.Team, existingMapping map[string]string, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(teams)

	// Copy existing mappings
	for k, v := range existingMapping {
		mapping[k] = v
	}

	for idx, team := range teams {
		if progress != nil {
			progress("spaces", idx+1, total, team.DisplayName)
		}

		// Skip deleted teams
		if team.IsDeleted() {
			stats.SpacesSkipped++
			continue
		}

		// Skip if already imported (exists in mapping)
		if _, exists := existingMapping[team.ID]; exists {
			logger.Info("Space '%s' already imported, skipped", team.DisplayName)
			stats.SpacesSkipped++
			continue
		}

		// Create space
		resp, err := i.client.CreateSpace(team.DisplayName, team.Description, team.IsOpen())
		if err != nil {
			logger.Error("Failed to create space '%s': %v", team.DisplayName, err)
			stats.SpacesFailed++
			continue
		}

		logger.Success("Created space '%s' -> %s", team.DisplayName, resp.RoomID)
		mapping[team.ID] = resp.RoomID
		stats.SpacesCreated++
	}

	return mapping, stats, nil
}

// ImportChannelsAsRooms imports channels from Mattermost as Matrix rooms
func (i *Importer) ImportChannelsAsRooms(channels []mattermost.Channel, existingMapping map[string]string, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(channels)

	// Copy existing mappings
	for k, v := range existingMapping {
		mapping[k] = v
	}

	for idx, channel := range channels {
		if progress != nil {
			progress("rooms", idx+1, total, channel.DisplayName)
		}

		// Skip deleted channels
		if channel.IsDeleted() {
			stats.RoomsSkipped++
			continue
		}

		// Skip direct messages and group messages
		if channel.IsDirect() || channel.IsGroup() {
			stats.RoomsSkipped++
			continue
		}

		// Skip if already imported (exists in mapping)
		if _, exists := existingMapping[channel.ID]; exists {
			logger.Info("Room '%s' already imported, skipped", channel.DisplayName)
			stats.RoomsSkipped++
			continue
		}

		// Create room
		topic := channel.Purpose
		if topic == "" {
			topic = channel.Header
		}

		resp, err := i.client.CreateRegularRoom(channel.DisplayName, topic, channel.IsPublic())
		if err != nil {
			logger.Error("Failed to create room '%s': %v", channel.DisplayName, err)
			stats.RoomsFailed++
			continue
		}

		logger.Success("Created room '%s' -> %s", channel.DisplayName, resp.RoomID)
		mapping[channel.ID] = resp.RoomID
		stats.RoomsCreated++
	}

	return mapping, stats, nil
}

// ApplyTeamMemberships invites users to spaces based on team memberships
func (i *Importer) ApplyTeamMemberships(
	memberships []mattermost.TeamMember,
	userMapping map[string]string,
	spaceMapping map[string]string,
	progress ImportProgressCallback,
) (*ImportStats, error) {
	stats := &ImportStats{}
	total := len(memberships)

	for idx, membership := range memberships {
		if progress != nil {
			progress("team_memberships", idx+1, total, "")
		}

		// Skip deleted memberships
		if membership.IsDeleted() {
			stats.MembersSkipped++
			continue
		}

		// Get Matrix IDs
		userID, userExists := userMapping[membership.UserID]
		spaceID, spaceExists := spaceMapping[membership.TeamID]

		if !userExists || !spaceExists {
			if !userExists {
				logger.Warn("Team membership skipped: user %s not in mapping", membership.UserID)
			}
			if !spaceExists {
				logger.Warn("Team membership skipped: team %s not in mapping", membership.TeamID)
			}
			stats.MembersSkipped++
			continue
		}

		// Invite user to space
		if err := i.client.InviteUser(spaceID, userID); err != nil {
			logger.Error("Failed to invite %s to space %s: %v", userID, spaceID, err)
			stats.MembersFailed++
			continue
		}

		stats.MembersAdded++
	}

	return stats, nil
}

// ApplyChannelMemberships invites users to rooms based on channel memberships
func (i *Importer) ApplyChannelMemberships(
	memberships []mattermost.ChannelMember,
	userMapping map[string]string,
	roomMapping map[string]string,
	progress ImportProgressCallback,
) (*ImportStats, error) {
	stats := &ImportStats{}
	total := len(memberships)

	for idx, membership := range memberships {
		if progress != nil {
			progress("channel_memberships", idx+1, total, "")
		}

		// Get Matrix IDs
		userID, userExists := userMapping[membership.UserID]
		roomID, roomExists := roomMapping[membership.ChannelID]

		if !userExists || !roomExists {
			if !userExists {
				logger.Warn("Channel membership skipped: user %s not in mapping", membership.UserID)
			}
			if !roomExists {
				logger.Warn("Channel membership skipped: channel %s not in mapping", membership.ChannelID)
			}
			stats.MembersSkipped++
			continue
		}

		// Invite user to room
		if err := i.client.InviteUser(roomID, userID); err != nil {
			logger.Error("Failed to invite %s to room %s: %v", userID, roomID, err)
			stats.MembersFailed++
			continue
		}

		stats.MembersAdded++
	}

	return stats, nil
}

// LinkRoomsToSpaces links rooms to their parent spaces based on channel-team relationships
func (i *Importer) LinkRoomsToSpaces(
	channels []mattermost.Channel,
	spaceMapping map[string]string,
	roomMapping map[string]string,
	progress ImportProgressCallback,
) (*ImportStats, error) {
	stats := &ImportStats{}
	total := len(channels)

	for idx, channel := range channels {
		if progress != nil {
			progress("linking", idx+1, total, channel.DisplayName)
		}

		// Skip if no team association
		if channel.TeamID == "" {
			continue
		}

		// Get Matrix IDs
		spaceID, spaceExists := spaceMapping[channel.TeamID]
		roomID, roomExists := roomMapping[channel.ID]

		if !spaceExists || !roomExists {
			continue
		}

		// Add room as child of space
		if err := i.client.AddRoomToSpace(spaceID, roomID, true); err != nil {
			logger.Error("Failed to link room '%s' to space: %v", channel.DisplayName, err)
			stats.RoomsLinkFailed++
			continue
		}

		// Set space as parent of room
		if err := i.client.SetRoomParent(roomID, spaceID, true); err != nil {
			// Non-critical error, room is still linked as child
			logger.Warn("Failed to set parent for room '%s': %v", channel.DisplayName, err)
		}

		logger.Success("Linked room '%s' to space", channel.DisplayName)
		stats.RoomsLinked++
	}

	return stats, nil
}

// ImportAssetsResult holds the result of importing assets
type ImportAssetsResult struct {
	UserMapping  map[string]string
	SpaceMapping map[string]string
	RoomMapping  map[string]string
	Stats        *ImportStats
}

// ExistingMappings holds existing mappings to skip already imported items
type ExistingMappings struct {
	Users    map[string]string
	Spaces   map[string]string
	Rooms    map[string]string
}

// ImportAssets imports all assets (users, teams as spaces, channels as rooms)
// If existingMappings is provided, already imported items will be skipped
func (i *Importer) ImportAssets(assets *mattermost.Assets, existingMappings *ExistingMappings, progress ImportProgressCallback) (*ImportAssetsResult, error) {
	result := &ImportAssetsResult{
		Stats: &ImportStats{},
	}

	logger.Info("=== ImportAssets Started ===")
	logger.Info("Assets to import: %d users, %d teams, %d channels", 
		len(assets.Users), len(assets.Teams), len(assets.Channels))

	// Initialize empty mappings if not provided
	if existingMappings == nil {
		existingMappings = &ExistingMappings{
			Users:  make(map[string]string),
			Spaces: make(map[string]string),
			Rooms:  make(map[string]string),
		}
		logger.Info("No existing mappings provided, starting fresh")
	} else {
		logger.Info("Existing mappings: %d users, %d spaces, %d rooms",
			len(existingMappings.Users), len(existingMappings.Spaces), len(existingMappings.Rooms))
	}

	// Import users
	logger.Info("=== Starting User Import ===")
	userMapping, userStats, err := i.ImportUsers(assets.Users, existingMappings.Users, progress)
	if err != nil {
		logger.Error("User import failed: %v", err)
		return nil, fmt.Errorf("failed to import users: %w", err)
	}
	result.UserMapping = userMapping
	result.Stats.UsersCreated = userStats.UsersCreated
	result.Stats.UsersSkipped = userStats.UsersSkipped
	result.Stats.UsersFailed = userStats.UsersFailed
	logger.Info("User import completed: created=%d, skipped=%d, failed=%d",
		userStats.UsersCreated, userStats.UsersSkipped, userStats.UsersFailed)

	// Import teams as spaces
	spaceMapping, spaceStats, err := i.ImportTeamsAsSpaces(assets.Teams, existingMappings.Spaces, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import teams: %w", err)
	}
	result.SpaceMapping = spaceMapping
	result.Stats.SpacesCreated = spaceStats.SpacesCreated
	result.Stats.SpacesSkipped = spaceStats.SpacesSkipped
	result.Stats.SpacesFailed = spaceStats.SpacesFailed

	// Import channels as rooms
	roomMapping, roomStats, err := i.ImportChannelsAsRooms(assets.Channels, existingMappings.Rooms, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import channels: %w", err)
	}
	result.RoomMapping = roomMapping
	result.Stats.RoomsCreated = roomStats.RoomsCreated
	result.Stats.RoomsSkipped = roomStats.RoomsSkipped
	result.Stats.RoomsFailed = roomStats.RoomsFailed

	return result, nil
}

