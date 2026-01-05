package matrix

import (
	"fmt"
	"strings"

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
func (i *Importer) ImportUsers(users []mattermost.User, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(users)

	for idx, user := range users {
		if progress != nil {
			progress("users", idx+1, total, user.Username)
		}

		// Skip deleted users
		if user.IsDeleted() {
			stats.UsersSkipped++
			continue
		}

		// Check if user already exists
		exists, err := i.client.UserExists(user.Username)
		if err != nil {
			stats.UsersFailed++
			continue
		}

		if exists {
			// User already exists, just add to mapping
			mapping[user.ID] = i.client.FormatUserID(user.Username)
			stats.UsersSkipped++
			continue
		}

		// Create the user
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
			stats.UsersFailed++
			continue
		}

		mapping[user.ID] = resp.UserID
		stats.UsersCreated++
	}

	return mapping, stats, nil
}

// ImportTeamsAsSpaces imports teams from Mattermost as Matrix spaces
func (i *Importer) ImportTeamsAsSpaces(teams []mattermost.Team, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(teams)

	for idx, team := range teams {
		if progress != nil {
			progress("spaces", idx+1, total, team.DisplayName)
		}

		// Skip deleted teams
		if team.IsDeleted() {
			stats.SpacesSkipped++
			continue
		}

		// Create space
		resp, err := i.client.CreateSpace(team.DisplayName, team.Description, team.IsOpen())
		if err != nil {
			stats.SpacesFailed++
			continue
		}

		mapping[team.ID] = resp.RoomID
		stats.SpacesCreated++
	}

	return mapping, stats, nil
}

// ImportChannelsAsRooms imports channels from Mattermost as Matrix rooms
func (i *Importer) ImportChannelsAsRooms(channels []mattermost.Channel, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	mapping := make(map[string]string)
	stats := &ImportStats{}
	total := len(channels)

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

		// Create room
		topic := channel.Purpose
		if topic == "" {
			topic = channel.Header
		}

		resp, err := i.client.CreateRegularRoom(channel.DisplayName, topic, channel.IsPublic())
		if err != nil {
			stats.RoomsFailed++
			continue
		}

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
			stats.MembersSkipped++
			continue
		}

		// Invite user to space
		if err := i.client.InviteUser(spaceID, userID); err != nil {
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
			stats.MembersSkipped++
			continue
		}

		// Invite user to room
		if err := i.client.InviteUser(roomID, userID); err != nil {
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
			stats.RoomsLinkFailed++
			continue
		}

		// Set space as parent of room
		if err := i.client.SetRoomParent(roomID, spaceID, true); err != nil {
			// Non-critical error, room is still linked as child
		}

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

// ImportAssets imports all assets (users, teams as spaces, channels as rooms)
func (i *Importer) ImportAssets(assets *mattermost.Assets, progress ImportProgressCallback) (*ImportAssetsResult, error) {
	result := &ImportAssetsResult{
		Stats: &ImportStats{},
	}

	// Import users
	userMapping, userStats, err := i.ImportUsers(assets.Users, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import users: %w", err)
	}
	result.UserMapping = userMapping
	result.Stats.UsersCreated = userStats.UsersCreated
	result.Stats.UsersSkipped = userStats.UsersSkipped
	result.Stats.UsersFailed = userStats.UsersFailed

	// Import teams as spaces
	spaceMapping, spaceStats, err := i.ImportTeamsAsSpaces(assets.Teams, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import teams: %w", err)
	}
	result.SpaceMapping = spaceMapping
	result.Stats.SpacesCreated = spaceStats.SpacesCreated
	result.Stats.SpacesSkipped = spaceStats.SpacesSkipped
	result.Stats.SpacesFailed = spaceStats.SpacesFailed

	// Import channels as rooms
	roomMapping, roomStats, err := i.ImportChannelsAsRooms(assets.Channels, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import channels: %w", err)
	}
	result.RoomMapping = roomMapping
	result.Stats.RoomsCreated = roomStats.RoomsCreated
	result.Stats.RoomsSkipped = roomStats.RoomsSkipped
	result.Stats.RoomsFailed = roomStats.RoomsFailed

	return result, nil
}

