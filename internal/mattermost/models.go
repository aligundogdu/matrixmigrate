package mattermost

import "time"

// User represents a Mattermost user
type User struct {
	ID        string    `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	FirstName string    `json:"first_name" db:"firstname"`
	LastName  string    `json:"last_name" db:"lastname"`
	Nickname  string    `json:"nickname" db:"nickname"`
	Position  string    `json:"position" db:"position"`
	Locale    string    `json:"locale" db:"locale"`
	Timezone  string    `json:"timezone" db:"timezone"`
	CreateAt  int64     `json:"create_at" db:"createat"`
	UpdateAt  int64     `json:"update_at" db:"updateat"`
	DeleteAt  int64     `json:"delete_at" db:"deleteat"`
	Roles     string    `json:"roles" db:"roles"`
}

// IsDeleted returns true if the user is deleted
func (u *User) IsDeleted() bool {
	return u.DeleteAt > 0
}

// CreatedTime returns the creation time as time.Time
func (u *User) CreatedTime() time.Time {
	return time.UnixMilli(u.CreateAt)
}

// Team represents a Mattermost team (workspace)
type Team struct {
	ID              string `json:"id" db:"id"`
	Name            string `json:"name" db:"name"`
	DisplayName     string `json:"display_name" db:"displayname"`
	Description     string `json:"description" db:"description"`
	Email           string `json:"email" db:"email"`
	Type            string `json:"type" db:"type"`
	CompanyName     string `json:"company_name" db:"companyname"`
	AllowedDomains  string `json:"allowed_domains" db:"alloweddomains"`
	InviteID        string `json:"invite_id" db:"inviteid"`
	AllowOpenInvite bool   `json:"allow_open_invite" db:"allowopeninvite"`
	CreateAt        int64  `json:"create_at" db:"createat"`
	UpdateAt        int64  `json:"update_at" db:"updateat"`
	DeleteAt        int64  `json:"delete_at" db:"deleteat"`
}

// IsDeleted returns true if the team is deleted
func (t *Team) IsDeleted() bool {
	return t.DeleteAt > 0
}

// IsOpen returns true if the team is open (public)
func (t *Team) IsOpen() bool {
	return t.Type == "O"
}

// Channel represents a Mattermost channel
type Channel struct {
	ID          string `json:"id" db:"id"`
	TeamID      string `json:"team_id" db:"teamid"`
	Name        string `json:"name" db:"name"`
	DisplayName string `json:"display_name" db:"displayname"`
	Header      string `json:"header" db:"header"`
	Purpose     string `json:"purpose" db:"purpose"`
	Type        string `json:"type" db:"type"`
	CreateAt    int64  `json:"create_at" db:"createat"`
	UpdateAt    int64  `json:"update_at" db:"updateat"`
	DeleteAt    int64  `json:"delete_at" db:"deleteat"`
	CreatorID   string `json:"creator_id" db:"creatorid"`
	TotalMsgCount int64 `json:"total_msg_count" db:"totalmsgcount"`
}

// IsDeleted returns true if the channel is deleted
func (c *Channel) IsDeleted() bool {
	return c.DeleteAt > 0
}

// IsPublic returns true if the channel is public
func (c *Channel) IsPublic() bool {
	return c.Type == "O"
}

// IsPrivate returns true if the channel is private
func (c *Channel) IsPrivate() bool {
	return c.Type == "P"
}

// IsDirect returns true if the channel is a direct message
func (c *Channel) IsDirect() bool {
	return c.Type == "D"
}

// IsGroup returns true if the channel is a group message
func (c *Channel) IsGroup() bool {
	return c.Type == "G"
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	TeamID   string `json:"team_id" db:"teamid"`
	UserID   string `json:"user_id" db:"userid"`
	Roles    string `json:"roles" db:"roles"`
	DeleteAt int64  `json:"delete_at" db:"deleteat"`
	CreateAt int64  `json:"create_at" db:"-"` // May not exist in older versions
}

// IsDeleted returns true if the membership is deleted
func (tm *TeamMember) IsDeleted() bool {
	return tm.DeleteAt > 0
}

// IsAdmin returns true if the member has admin role
func (tm *TeamMember) IsAdmin() bool {
	return tm.Roles == "team_admin" || tm.Roles == "team_admin team_user"
}

// ChannelMember represents a user's membership in a channel
type ChannelMember struct {
	ChannelID   string `json:"channel_id" db:"channelid"`
	UserID      string `json:"user_id" db:"userid"`
	Roles       string `json:"roles" db:"roles"`
	NotifyProps string `json:"notify_props" db:"notifyprops"`
	LastViewedAt int64 `json:"last_viewed_at" db:"lastviewedat"`
	MsgCount    int64  `json:"msg_count" db:"msgcount"`
}

// IsAdmin returns true if the member has admin role
func (cm *ChannelMember) IsAdmin() bool {
	return cm.Roles == "channel_admin" || cm.Roles == "channel_admin channel_user"
}

// Assets represents all exportable data from Mattermost
type Assets struct {
	ExportedAt int64     `json:"exported_at"`
	Version    string    `json:"version"`
	Users      []User    `json:"users"`
	Teams      []Team    `json:"teams"`
	Channels   []Channel `json:"channels"`
}

// Memberships represents all membership data from Mattermost
type Memberships struct {
	ExportedAt      int64           `json:"exported_at"`
	Version         string          `json:"version"`
	TeamMembers     []TeamMember    `json:"team_members"`
	ChannelMembers  []ChannelMember `json:"channel_members"`
}

// ExportStats holds statistics about an export
type ExportStats struct {
	UsersTotal      int `json:"users_total"`
	UsersActive     int `json:"users_active"`
	TeamsTotal      int `json:"teams_total"`
	TeamsActive     int `json:"teams_active"`
	ChannelsTotal   int `json:"channels_total"`
	ChannelsActive  int `json:"channels_active"`
	ChannelsPublic  int `json:"channels_public"`
	ChannelsPrivate int `json:"channels_private"`
}

// CalculateStats calculates export statistics from assets
func (a *Assets) CalculateStats() ExportStats {
	stats := ExportStats{
		UsersTotal:    len(a.Users),
		TeamsTotal:    len(a.Teams),
		ChannelsTotal: len(a.Channels),
	}

	for _, u := range a.Users {
		if !u.IsDeleted() {
			stats.UsersActive++
		}
	}

	for _, t := range a.Teams {
		if !t.IsDeleted() {
			stats.TeamsActive++
		}
	}

	for _, c := range a.Channels {
		if !c.IsDeleted() {
			stats.ChannelsActive++
			if c.IsPublic() {
				stats.ChannelsPublic++
			} else if c.IsPrivate() {
				stats.ChannelsPrivate++
			}
		}
	}

	return stats
}

// Post represents a Mattermost message/post
type Post struct {
	ID        string `json:"id" db:"id"`
	CreateAt  int64  `json:"create_at" db:"createat"`
	UpdateAt  int64  `json:"update_at" db:"updateat"`
	DeleteAt  int64  `json:"delete_at" db:"deleteat"`
	UserID    string `json:"user_id" db:"userid"`
	ChannelID string `json:"channel_id" db:"channelid"`
	RootID    string `json:"root_id" db:"rootid"`       // Parent post ID for replies/threads
	OriginalID string `json:"original_id" db:"originalid"` // Original post ID if edited
	Message   string `json:"message" db:"message"`
	Type      string `json:"type" db:"type"`           // "" for normal, "system_*" for system messages
	Props     string `json:"props" db:"props"`         // JSON string with additional properties
	FileIDs   string `json:"file_ids" db:"fileids"`    // JSON array of file IDs
}

// FileInfo represents a Mattermost file attachment
type FileInfo struct {
	ID              string `json:"id" db:"id"`
	CreatorID       string `json:"creator_id" db:"creatorid"`
	PostID          string `json:"post_id" db:"postid"`
	CreateAt        int64  `json:"create_at" db:"createat"`
	UpdateAt        int64  `json:"update_at" db:"updateat"`
	DeleteAt        int64  `json:"delete_at" db:"deleteat"`
	Path            string `json:"path" db:"path"`               // Relative path in storage
	ThumbnailPath   string `json:"thumbnail_path" db:"thumbnailpath"`
	PreviewPath     string `json:"preview_path" db:"previewpath"`
	Name            string `json:"name" db:"name"`               // Original filename
	Extension       string `json:"extension" db:"extension"`     // File extension
	Size            int64  `json:"size" db:"size"`               // File size in bytes
	MimeType        string `json:"mime_type" db:"mimetype"`
	Width           int    `json:"width" db:"width"`             // For images
	Height          int    `json:"height" db:"height"`           // For images
	HasPreviewImage bool   `json:"has_preview_image" db:"haspreviewimage"`
	MiniPreview     []byte `json:"mini_preview,omitempty" db:"minipreview"` // Base64 thumbnail
}

// IsDeleted returns true if the file is deleted
func (f *FileInfo) IsDeleted() bool {
	return f.DeleteAt > 0
}

// IsImage returns true if the file is an image
func (f *FileInfo) IsImage() bool {
	switch f.MimeType {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp", "image/bmp":
		return true
	}
	return false
}

// IsVideo returns true if the file is a video
func (f *FileInfo) IsVideo() bool {
	switch f.MimeType {
	case "video/mp4", "video/webm", "video/ogg", "video/quicktime", "video/x-msvideo":
		return true
	}
	return false
}

// IsAudio returns true if the file is an audio file
func (f *FileInfo) IsAudio() bool {
	switch f.MimeType {
	case "audio/mpeg", "audio/mp3", "audio/ogg", "audio/wav", "audio/webm", "audio/flac":
		return true
	}
	return false
}

// GetMatrixMsgType returns the Matrix message type for this file
func (f *FileInfo) GetMatrixMsgType() string {
	if f.IsImage() {
		return "m.image"
	}
	if f.IsVideo() {
		return "m.video"
	}
	if f.IsAudio() {
		return "m.audio"
	}
	return "m.file"
}

// CreatedTime returns the creation time as time.Time
func (f *FileInfo) CreatedTime() time.Time {
	return time.UnixMilli(f.CreateAt)
}

// Files represents exported file data from Mattermost
type Files struct {
	ExportedAt int64      `json:"exported_at"`
	Version    string     `json:"version"`
	Files      []FileInfo `json:"files"`
}

// FileStats holds statistics about files
type FileStats struct {
	TotalFiles   int            `json:"total_files"`
	TotalSize    int64          `json:"total_size"`
	Images       int            `json:"images"`
	Videos       int            `json:"videos"`
	Audio        int            `json:"audio"`
	Documents    int            `json:"documents"`
	ByExtension  map[string]int `json:"by_extension"`
}

// CalculateFileStats calculates file statistics
func (f *Files) CalculateFileStats() FileStats {
	stats := FileStats{
		TotalFiles:  len(f.Files),
		ByExtension: make(map[string]int),
	}

	for _, file := range f.Files {
		stats.TotalSize += file.Size
		stats.ByExtension[file.Extension]++
		
		if file.IsImage() {
			stats.Images++
		} else if file.IsVideo() {
			stats.Videos++
		} else if file.IsAudio() {
			stats.Audio++
		} else {
			stats.Documents++
		}
	}

	return stats
}

// IsDeleted returns true if the post is deleted
func (p *Post) IsDeleted() bool {
	return p.DeleteAt > 0
}

// IsReply returns true if the post is a reply to another post
func (p *Post) IsReply() bool {
	return p.RootID != ""
}

// IsSystemMessage returns true if the post is a system-generated message
func (p *Post) IsSystemMessage() bool {
	return p.Type != "" && len(p.Type) > 0
}

// CreatedTime returns the creation time as time.Time
func (p *Post) CreatedTime() time.Time {
	return time.UnixMilli(p.CreateAt)
}

// Messages represents all message data from Mattermost
type Messages struct {
	ExportedAt int64      `json:"exported_at"`
	Version    string     `json:"version"`
	Posts      []Post     `json:"posts"`
	Files      []FileInfo `json:"files,omitempty"` // File attachments
}

// MessageStats holds statistics about messages
type MessageStats struct {
	TotalPosts    int            `json:"total_posts"`
	ActivePosts   int            `json:"active_posts"`
	DeletedPosts  int            `json:"deleted_posts"`
	Replies       int            `json:"replies"`
	SystemPosts   int            `json:"system_posts"`
	ByChannel     map[string]int `json:"by_channel"`
}

// CalculateMessageStats calculates message statistics
func (m *Messages) CalculateMessageStats() MessageStats {
	stats := MessageStats{
		TotalPosts: len(m.Posts),
		ByChannel:  make(map[string]int),
	}

	for _, p := range m.Posts {
		if p.IsDeleted() {
			stats.DeletedPosts++
			continue
		}
		stats.ActivePosts++
		if p.IsReply() {
			stats.Replies++
		}
		if p.IsSystemMessage() {
			stats.SystemPosts++
		}
		stats.ByChannel[p.ChannelID]++
	}

	return stats
}





