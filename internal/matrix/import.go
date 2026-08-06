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
			Email:       strings.TrimSpace(user.Email),
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
func (i *Importer) ImportChannelsAsRooms(channels []mattermost.Channel, userMapping map[string]string, existingMapping map[string]string, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
	return i.ImportChannelsAsRoomsWithDMs(channels, userMapping, existingMapping, true, progress)
}

// ImportChannelsAsRoomsWithDMs imports channels from Mattermost as Matrix rooms with optional DM support
func (i *Importer) ImportChannelsAsRoomsWithDMs(channels []mattermost.Channel, userMapping map[string]string, existingMapping map[string]string, migrateDMs bool, progress ImportProgressCallback) (map[string]string, *ImportStats, error) {
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

		// Handle direct messages
		if channel.IsDirect() {
			if !migrateDMs {
				stats.DMRoomsSkipped++
				continue
			}
			roomID, err := i.importDMAsRoom(channel, userMapping, existingMapping)
			if err != nil {
				logger.Warn("Failed to import DM: %v", err)
				stats.DMRoomsFailed++
			} else if roomID != "" {
				mapping[channel.ID] = roomID
				stats.DMRoomsCreated++
				logger.Success("Created DM room -> %s", roomID)
			} else {
				stats.DMRoomsSkipped++
			}
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

		// Grant the channel creator admin power level (100) so they own the room.
		if creatorMxID, ok := userMapping[channel.CreatorID]; ok && creatorMxID != "" {
			if plErr := i.client.SetUserPowerLevel(resp.RoomID, creatorMxID, 100); plErr != nil {
				logger.Warn("Failed to set creator power level for room '%s': %v", channel.DisplayName, plErr)
			}
		}
	}

	return mapping, stats, nil
}

// matrixLocalpart returns the local part of a Matrix ID (@alice:domain -> alice).
func matrixLocalpart(mxid string) string {
	s := strings.TrimPrefix(strings.TrimSpace(mxid), "@")
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return s
	}
	return s[:idx]
}

// dmRoomDisplayTitle picks a Matrix room title for an imported DM before the migration user leaves.
func dmRoomDisplayTitle(channel mattermost.Channel, mxUserA, mxUserB string, selfDM bool) string {
	if n := strings.TrimSpace(channel.DisplayName); n != "" {
		return n
	}
	if selfDM {
		return matrixLocalpart(mxUserA) + " (You)"
	}
	return matrixLocalpart(mxUserA) + " & " + matrixLocalpart(mxUserB)
}

// importDMAsRoom imports a Mattermost DM channel as a Matrix DM room
// Returns the room ID if successful, empty string if skipped, error if failed
func (i *Importer) importDMAsRoom(channel mattermost.Channel, userMapping map[string]string, existingMapping map[string]string) (string, error) {
	// Skip if already imported
	if roomID, exists := existingMapping[channel.ID]; exists {
		logger.Info("DM room already imported, skipped")
		return roomID, nil
	}

	// Parse DM user IDs
	mmUserA, mmUserB, ok := channel.DMUserIDs()
	if !ok {
		return "", fmt.Errorf("invalid DM channel name format: %s", channel.Name)
	}

	// Look up both users in mapping
	mxUserA, existsA := userMapping[mmUserA]
	mxUserB, existsB := userMapping[mmUserB]

	if !existsA || !existsB {
		if !existsA {
			logger.Warn("DM user %s not in mapping, skipping DM", mmUserA)
		}
		if !existsB {
			logger.Warn("DM user %s not in mapping, skipping DM", mmUserB)
		}
		return "", nil
	}

	invites := []string{mxUserA, mxUserB}
	isSelfDM := mxUserA == mxUserB
	if isSelfDM {
		// Self-DM: Matrix createRoom must not list the same MXID twice in invite.
		invites = []string{mxUserA}
		logger.Info(
			"Creating self-DM room for Mattermost user %s -> Matrix %s (channel %s)",
			mmUserA, mxUserA, channel.ID,
		)
	}

	topic := channel.Purpose
	if topic == "" {
		topic = channel.Header
	}
	roomName := dmRoomDisplayTitle(channel, mxUserA, mxUserB, isSelfDM)

	// Create DM room with user(s) invited
	resp, err := i.client.CreateDMRoom(roomName, topic, invites)
	if err != nil {
		return "", fmt.Errorf("failed to create DM room: %w", err)
	}

	if isSelfDM {
		logger.Info("Created self-DM room %s for user %s", resp.RoomID, mxUserA)
	} else {
		logger.Info("Created DM room %s for users %s and %s", resp.RoomID, mxUserA, mxUserB)
	}

	// Force-join user(s) using Admin API (no invitation acceptance required)
	if err := i.client.ForceJoinUser(resp.RoomID, mxUserA); err != nil {
		logger.Warn("Failed to force-join %s to DM: %v", mxUserA, err)
		// Continue even if one user fails to join
	}
	if mxUserA != mxUserB {
		if err := i.client.ForceJoinUser(resp.RoomID, mxUserB); err != nil {
			logger.Warn("Failed to force-join %s to DM: %v", mxUserB, err)
			// Continue even if one user fails to join
		}
	}

	// Remove admin user from DM room (DMs should only contain the two participants)
	if err := i.client.LeaveRoom(resp.RoomID); err != nil {
		logger.Warn("Failed to remove admin from DM room: %v", err)
		// Non-critical error - room is still functional
	}

	return resp.RoomID, nil
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

	logger.Info("Starting team membership import: %d memberships to process", total)

	for idx, membership := range memberships {
		if progress != nil {
			progress("team_memberships", idx+1, total, "")
		}

		// Skip deleted memberships
		if membership.IsDeleted() {
			logger.Info("Team membership %d/%d: deleted, skipping", idx+1, total)
			stats.MembersSkipped++
			continue
		}

		// Get Matrix IDs
		userID, userExists := userMapping[membership.UserID]
		spaceID, spaceExists := spaceMapping[membership.TeamID]

		if !userExists || !spaceExists {
			if !userExists {
				logger.Warn("Team membership %d/%d skipped: user %s not in mapping", idx+1, total, membership.UserID)
			}
			if !spaceExists {
				logger.Warn("Team membership %d/%d skipped: team %s not in mapping", idx+1, total, membership.TeamID)
			}
			stats.MembersSkipped++
			continue
		}

		logger.Info("Team membership %d/%d: inviting %s to space %s", idx+1, total, userID, spaceID)

		// Invite user to space
		if err := i.client.InviteUser(spaceID, userID); err != nil {
			logger.Error("Team membership %d/%d failed: %s -> %s: %v", idx+1, total, userID, spaceID, err)
			stats.MembersFailed++
			continue
		}

		// Team admins get moderator power level (50) in the space.
		if membership.IsAdmin() {
			if plErr := i.client.SetUserPowerLevel(spaceID, userID, 50); plErr != nil {
				logger.Warn("Failed to set moderator power level for %s in space %s: %v", userID, spaceID, plErr)
			}
		}

		logger.Success("Team membership %d/%d: %s added to space", idx+1, total, userID)
		stats.MembersAdded++
	}

	logger.Info("Team membership import completed: added=%d, skipped=%d, failed=%d", 
		stats.MembersAdded, stats.MembersSkipped, stats.MembersFailed)

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

	logger.Info("Starting channel membership import: %d memberships to process", total)

	for idx, membership := range memberships {
		if progress != nil {
			progress("channel_memberships", idx+1, total, "")
		}

		// Get Matrix IDs
		userID, userExists := userMapping[membership.UserID]
		roomID, roomExists := roomMapping[membership.ChannelID]

		if !userExists || !roomExists {
			if !userExists {
				logger.Warn("Channel membership %d/%d skipped: user %s not in mapping", idx+1, total, membership.UserID)
			}
			if !roomExists {
				logger.Warn("Channel membership %d/%d skipped: channel %s not in mapping", idx+1, total, membership.ChannelID)
			}
			stats.MembersSkipped++
			continue
		}

		logger.Info("Channel membership %d/%d: inviting %s to room %s", idx+1, total, userID, roomID)

		// Invite user to room
		if err := i.client.InviteUser(roomID, userID); err != nil {
			logger.Error("Channel membership %d/%d failed: %s -> %s: %v", idx+1, total, userID, roomID, err)
			stats.MembersFailed++
			continue
		}

		// Channel admins get moderator power level (50) so they can manage the room.
		if membership.IsAdmin() {
			if plErr := i.client.SetUserPowerLevel(roomID, userID, 50); plErr != nil {
				logger.Warn("Failed to set moderator power level for %s in %s: %v", userID, roomID, plErr)
			}
		}

		logger.Success("Channel membership %d/%d: %s added to room", idx+1, total, userID)
		stats.MembersAdded++
	}

	logger.Info("Channel membership import completed: added=%d, skipped=%d, failed=%d", 
		stats.MembersAdded, stats.MembersSkipped, stats.MembersFailed)

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

// LeaveAllRooms makes the migration admin user leave all created rooms and spaces.
// Should be called after all memberships have been applied.
func (i *Importer) LeaveAllRooms(
	spaceMapping map[string]string,
	roomMapping map[string]string,
	progress ImportProgressCallback,
) *ImportStats {
	stats := &ImportStats{}

	// Collect unique room/space IDs
	roomIDs := make(map[string]bool)
	for _, id := range spaceMapping {
		roomIDs[id] = true
	}
	for _, id := range roomMapping {
		roomIDs[id] = true
	}

	total := len(roomIDs)
	idx := 0
	for roomID := range roomIDs {
		idx++
		if progress != nil {
			progress("leave_rooms", idx, total, roomID)
		}
		if err := i.client.LeaveRoom(roomID); err != nil {
			logger.Warn("Failed to leave room %s: %v", roomID, err)
			stats.RoomsLeftFailed++
		} else {
			logger.Info("Admin left room %s", roomID)
			stats.RoomsLeft++
		}
	}

	logger.Info("Admin user left %d rooms (%d failed)", stats.RoomsLeft, stats.RoomsLeftFailed)
	return stats
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
	return i.ImportAssetsWithDMs(assets, existingMappings, true, progress)
}

// ImportAssetsWithDMs imports all assets with optional DM support
func (i *Importer) ImportAssetsWithDMs(assets *mattermost.Assets, existingMappings *ExistingMappings, migrateDMs bool, progress ImportProgressCallback) (*ImportAssetsResult, error) {
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
	roomMapping, roomStats, err := i.ImportChannelsAsRoomsWithDMs(assets.Channels, userMapping, existingMappings.Rooms, migrateDMs, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to import channels: %w", err)
	}
	result.RoomMapping = roomMapping
	result.Stats.RoomsCreated = roomStats.RoomsCreated
	result.Stats.RoomsSkipped = roomStats.RoomsSkipped
	result.Stats.RoomsFailed = roomStats.RoomsFailed
	result.Stats.DMRoomsCreated = roomStats.DMRoomsCreated
	result.Stats.DMRoomsSkipped = roomStats.DMRoomsSkipped
	result.Stats.DMRoomsFailed = roomStats.DMRoomsFailed

	return result, nil
}

// MessageImportStats holds statistics about message import
type MessageImportStats struct {
	MessagesImported int `json:"messages_imported"`
	MessagesSkipped  int `json:"messages_skipped"` // Already imported
	MessagesFailed   int `json:"messages_failed"`
	RepliesImported  int `json:"replies_imported"`
	RepliesFailed    int `json:"replies_failed"`  // Reply target not found
	FilesLinked      int `json:"files_linked"`    // Files added as links
	FilesUploaded    int `json:"files_uploaded"`  // Files uploaded to Matrix
	FilesSkipped     int `json:"files_skipped"`   // Files skipped
}

// FileConfig holds file migration settings
type FileConfig struct {
	Mode          string // "link", "upload", or "skip"
	S3PublicURL   string // Base URL for S3 files
	MaxUploadSize int64  // Max file size for upload in bytes
	LocalDataPath string // Base path for file storage on the Mattermost server
	// ReadFile reads raw bytes from a file path. Injected by the orchestrator:
	// os.ReadFile for local mode, RemoteExecutor.ReadFile for SSH mode.
	ReadFile func(path string) ([]byte, error)
	// UsernameToMxID maps Mattermost usernames to Matrix user IDs for @mention pills.
	UsernameToMxID map[string]string
}

// MessageImportCallback is called for each message imported
type MessageImportCallback func(current, total int, channelName string, status string)

func logMmSendFailure(op string, post mattermost.Post, roomID, senderMxID string, err error) {
	logger.Error("%s: post=%s channel=%s room=%s mm_user=%s sender_mx=%q: %v",
		op, post.ID, post.ChannelID, roomID, post.UserID, senderMxID, err)
}

// ImportMessagesResult contains the result of message import
type ImportMessagesResult struct {
	Stats    *MessageImportStats
	Mapping  map[string]string // MattermostID -> MatrixEventID
	Errors   []string
}

// ImportMessages imports messages from Mattermost posts to Matrix rooms
// This requires Application Service token for timestamp support
func (i *Importer) ImportMessages(
	posts []mattermost.Post,
	channelToRoom map[string]string,  // Mattermost channel ID -> Matrix room ID
	userMapping map[string]string,    // Mattermost user ID -> Matrix user ID
	existingMapping map[string]string, // Mattermost post ID -> Matrix event ID (for resume)
	usernameToMxID map[string]string, // Mattermost username -> Matrix user ID (for @mention pills)
	progress MessageImportCallback,
) (*ImportMessagesResult, error) {
	result := &ImportMessagesResult{
		Stats:   &MessageImportStats{},
		Mapping: make(map[string]string),
		Errors:  []string{},
	}
	
	if !i.client.HasASToken() {
		logger.Warn("No Application Service token configured - messages will be imported without original timestamps")
	}
	
	total := len(posts)
	logger.Info("Starting message import: %d posts to process", total)
	
	// Collect all existing mappings
	for k, v := range existingMapping {
		result.Mapping[k] = v
	}
	
	// Sort posts by timestamp (they should already be sorted, but just in case)
	// This ensures parent messages are imported before replies
	
	// Process messages in order
	for idx, post := range posts {
		// Check if already imported
		if _, exists := existingMapping[post.ID]; exists {
			result.Stats.MessagesSkipped++
			if progress != nil {
				progress(idx+1, total, post.ChannelID, "skipped")
			}
			continue
		}
		
		// Get target room
		roomID, roomExists := channelToRoom[post.ChannelID]
		if !roomExists {
			result.Stats.MessagesFailed++
			errTxt := fmt.Sprintf("No room mapping for channel %s (post %s)", post.ChannelID, post.ID)
			result.Errors = append(result.Errors, errTxt)
			logger.Error("message import no room mapping: %s", errTxt)
			if progress != nil {
				progress(idx+1, total, post.ChannelID, "failed:no_room")
			}
			continue
		}
		
		// Get sender
		senderID, userExists := userMapping[post.UserID]
		if !userExists {
			// Fall back to empty sender (will use AS bot)
			senderID = ""
			logger.Warn("No user mapping for user %s, message will be sent as AS bot", post.UserID)
		}
		
		// Handle reply
		var eventID string
		
		_, formattedBody := FormatMessage(post.Message, usernameToMxID)

		if post.IsReply() {
			parentEventID, parentExists := result.Mapping[post.RootID]
			if !parentExists {
				result.Stats.RepliesFailed++
				parentErr := fmt.Sprintf("Parent post %s not found for reply %s", post.RootID, post.ID)
				result.Errors = append(result.Errors, parentErr)
				logger.Warn("message import %s — sending reply body as standalone message instead", parentErr)

				resp, sendErr := i.client.SendMessageWithTimestamp(roomID, post.Message, formattedBody, post.CreateAt, senderID)
				if sendErr != nil {
					result.Stats.MessagesFailed++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to send message %s: %v", post.ID, sendErr))
					logMmSendFailure("SendMessage(repair orphan reply)", post, roomID, senderID, sendErr)
					if progress != nil {
						progress(idx+1, total, post.ChannelID, "failed:send_error")
					}
					continue
				}
				eventID = resp.EventID
			} else {
				resp, sendErr := i.client.SendReplyWithTimestamp(roomID, post.Message, formattedBody, parentEventID, post.CreateAt, senderID)
				if sendErr != nil {
					result.Stats.RepliesFailed++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to send reply %s: %v", post.ID, sendErr))
					logMmSendFailure("SendReply", post, roomID, senderID, sendErr)
					if progress != nil {
						progress(idx+1, total, post.ChannelID, "failed:reply_error")
					}
					continue
				}
				eventID = resp.EventID
				result.Stats.RepliesImported++
			}
		} else {
			resp, sendErr := i.client.SendMessageWithTimestamp(roomID, post.Message, formattedBody, post.CreateAt, senderID)
			if sendErr != nil {
				result.Stats.MessagesFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to send message %s: %v", post.ID, sendErr))
				logMmSendFailure("SendMessage", post, roomID, senderID, sendErr)
				if progress != nil {
					progress(idx+1, total, post.ChannelID, "failed:send_error")
				}
				continue
			}
			eventID = resp.EventID
		}

		result.Mapping[post.ID] = eventID
		result.Stats.MessagesImported++

		if progress != nil {
			progress(idx+1, total, post.ChannelID, "imported")
		}

		if (idx+1)%100 == 0 {
			logger.Info("Message import progress: %d/%d (%.1f%%)", idx+1, total, float64(idx+1)/float64(total)*100)
		}
	}

	logger.Info("Message import completed: imported=%d, skipped=%d, failed=%d, replies=%d",
		result.Stats.MessagesImported, result.Stats.MessagesSkipped,
		result.Stats.MessagesFailed, result.Stats.RepliesImported)

	return result, nil
}

// ImportMessagesWithFiles imports messages with file attachments
// filesByPost maps post ID to list of file infos
func (i *Importer) ImportMessagesWithFiles(
	posts []mattermost.Post,
	channelToRoom map[string]string,
	userMapping map[string]string,
	existingMapping map[string]string,
	filesByPost map[string][]mattermost.FileInfo,
	fileConfig *FileConfig,
	progress MessageImportCallback,
) (*ImportMessagesResult, error) {
	result := &ImportMessagesResult{
		Stats:   &MessageImportStats{},
		Mapping: make(map[string]string),
		Errors:  []string{},
	}
	
	if !i.client.HasASToken() {
		logger.Warn("No Application Service token configured - messages will be imported without original timestamps")
	}
	
	total := len(posts)
	logger.Info("Starting message import with files: %d posts to process", total)
	
	// Default file config
	if fileConfig == nil {
		fileConfig = &FileConfig{Mode: "skip"}
	}
	
	// Collect all existing mappings
	for k, v := range existingMapping {
		result.Mapping[k] = v
	}
	
	// Process messages in order
	for idx, post := range posts {
		// Check if already imported
		if _, exists := existingMapping[post.ID]; exists {
			result.Stats.MessagesSkipped++
			if progress != nil {
				progress(idx+1, total, post.ChannelID, "skipped")
			}
			continue
		}
		
		// Get target room
		roomID, roomExists := channelToRoom[post.ChannelID]
		if !roomExists {
			result.Stats.MessagesFailed++
			errTxt := fmt.Sprintf("No room mapping for channel %s (post %s)", post.ChannelID, post.ID)
			result.Errors = append(result.Errors, errTxt)
			logger.Error("message import no room mapping: %s", errTxt)
			if progress != nil {
				progress(idx+1, total, post.ChannelID, "failed:no_room")
			}
			continue
		}
		
		// Get sender
		senderID, userExists := userMapping[post.UserID]
		if !userExists {
			senderID = ""
			logger.Warn("No user mapping for user %s, message will be sent as AS bot", post.UserID)
		}
		
		// Build message content with files
		messageContent := post.Message
		files := filesByPost[post.ID]

		// "link" mode: embed S3 URLs directly into the text message
		if fileConfig.Mode == "link" && len(files) > 0 && fileConfig.S3PublicURL != "" {
			for _, file := range files {
				fileURL := strings.TrimSuffix(fileConfig.S3PublicURL, "/") + "/" + file.Path
				messageContent += fmt.Sprintf("\n\n📎 [%s](%s)", file.Name, fileURL)
				result.Stats.FilesLinked++
			}
		}

		// Render Markdown and resolve @mention pills for Matrix HTML rendering.
		_, formattedBody := FormatMessage(messageContent, fileConfig.UsernameToMxID)

		// Handle reply
		var eventID string

		if post.IsReply() {
			parentEventID, parentExists := result.Mapping[post.RootID]
			if !parentExists {
				result.Stats.RepliesFailed++
				parentErr := fmt.Sprintf("Parent post %s not found for reply %s", post.RootID, post.ID)
				result.Errors = append(result.Errors, parentErr)
				logger.Warn("message import %s — sending reply body as standalone message instead", parentErr)

				resp, sendErr := i.client.SendMessageWithTimestamp(roomID, messageContent, formattedBody, post.CreateAt, senderID)
				if sendErr != nil {
					result.Stats.MessagesFailed++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to send message %s: %v", post.ID, sendErr))
					logMmSendFailure("SendMessage(repair orphan reply)", post, roomID, senderID, sendErr)
					if progress != nil {
						progress(idx+1, total, post.ChannelID, "failed:send_error")
					}
					continue
				}
				eventID = resp.EventID
			} else {
				resp, sendErr := i.client.SendReplyWithTimestamp(roomID, messageContent, formattedBody, parentEventID, post.CreateAt, senderID)
				if sendErr != nil {
					result.Stats.RepliesFailed++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to send reply %s: %v", post.ID, sendErr))
					logMmSendFailure("SendReply", post, roomID, senderID, sendErr)
					if progress != nil {
						progress(idx+1, total, post.ChannelID, "failed:reply_error")
					}
					continue
				}
				eventID = resp.EventID
				result.Stats.RepliesImported++
			}
		} else {
			resp, sendErr := i.client.SendMessageWithTimestamp(roomID, messageContent, formattedBody, post.CreateAt, senderID)
			if sendErr != nil {
				result.Stats.MessagesFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to send message %s: %v", post.ID, sendErr))
				logMmSendFailure("SendMessage", post, roomID, senderID, sendErr)
				if progress != nil {
					progress(idx+1, total, post.ChannelID, "failed:send_error")
				}
				continue
			}
			eventID = resp.EventID
		}

		// "upload" mode: read each file from Mattermost storage and upload to Matrix.
		// Runs after the text message so the parent event already exists in the room.
		if fileConfig.Mode == "upload" && len(files) > 0 && fileConfig.ReadFile != nil {
			for _, file := range files {
				// Skip files that exceed the size limit
				if fileConfig.MaxUploadSize > 0 && file.Size > fileConfig.MaxUploadSize {
					logger.Warn("File %s (%d bytes) exceeds max upload size (%d bytes), skipping",
						file.Name, file.Size, fileConfig.MaxUploadSize)
					result.Stats.FilesSkipped++
					continue
				}

				// Build full path: localDataPath + "/" + relative path from DB
				fullPath := strings.TrimSuffix(fileConfig.LocalDataPath, "/") + "/" + file.Path

				data, readErr := fileConfig.ReadFile(fullPath)
				if readErr != nil {
					logger.Warn("Failed to read file %s (%s): %v — skipping", file.Name, fullPath, readErr)
					result.Stats.FilesSkipped++
					continue
				}

				mimeType := file.MimeType
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}

				uploadResp, uploadErr := i.client.UploadMedia(data, file.Name, mimeType)
				if uploadErr != nil {
					logger.Warn("Failed to upload file %s to Matrix: %v — skipping", file.Name, uploadErr)
					result.Stats.FilesSkipped++
					continue
				}

				fileContent := &FileMessageContent{
					MsgType:  file.GetMatrixMsgType(),
					Body:     file.Name,
					URL:      uploadResp.ContentURI,
					Filename: file.Name,
					Info: &FileInfo{
						MimeType: mimeType,
						Size:     file.Size,
						Width:    file.Width,
						Height:   file.Height,
					},
				}

				_, sendErr := i.client.SendFileMessage(roomID, fileContent, post.CreateAt, senderID)
				if sendErr != nil {
					logger.Warn("Failed to send file event for %s: %v — skipping", file.Name, sendErr)
					result.Stats.FilesSkipped++
					continue
				}

				logger.Success("File transferred to Matrix: %s (%d bytes) room=%s uri=%s", file.Name, len(data), roomID, uploadResp.ContentURI)
				result.Stats.FilesUploaded++
			}
		}

		// Store mapping
		result.Mapping[post.ID] = eventID
		result.Stats.MessagesImported++

		if progress != nil {
			progress(idx+1, total, post.ChannelID, "imported")
		}

		// Log progress every 100 messages
		if (idx+1)%100 == 0 {
			logger.Info("Message import progress: %d/%d (%.1f%%) - linked: %d, uploaded: %d, skipped: %d",
				idx+1, total, float64(idx+1)/float64(total)*100,
				result.Stats.FilesLinked, result.Stats.FilesUploaded, result.Stats.FilesSkipped)
		}
	}

	logger.Info("Message import completed: imported=%d, skipped=%d, failed=%d, replies=%d, files_linked=%d, files_uploaded=%d, files_skipped=%d",
		result.Stats.MessagesImported, result.Stats.MessagesSkipped,
		result.Stats.MessagesFailed, result.Stats.RepliesImported,
		result.Stats.FilesLinked, result.Stats.FilesUploaded, result.Stats.FilesSkipped)
	
	return result, nil
}

