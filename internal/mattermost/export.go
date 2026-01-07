package mattermost

import (
	"fmt"
	"time"
)

// Exporter handles exporting data from Mattermost
type Exporter struct {
	client *Client
}

// NewExporter creates a new exporter
func NewExporter(client *Client) *Exporter {
	return &Exporter{client: client}
}

// ExportProgressCallback is called to report export progress
type ExportProgressCallback func(stage string, current, total int)

// ExportAssets exports all assets (users, teams, channels)
func (e *Exporter) ExportAssets(progress ExportProgressCallback) (*Assets, error) {
	assets := &Assets{
		ExportedAt: time.Now().UnixMilli(),
		Version:    "1.0",
	}

	// Export users
	if progress != nil {
		progress("users", 0, 0)
	}
	users, err := e.client.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to export users: %w", err)
	}
	assets.Users = users
	if progress != nil {
		progress("users", len(users), len(users))
	}

	// Export teams
	if progress != nil {
		progress("teams", 0, 0)
	}
	teams, err := e.client.GetTeams()
	if err != nil {
		return nil, fmt.Errorf("failed to export teams: %w", err)
	}
	assets.Teams = teams
	if progress != nil {
		progress("teams", len(teams), len(teams))
	}

	// Export channels
	if progress != nil {
		progress("channels", 0, 0)
	}
	channels, err := e.client.GetChannels()
	if err != nil {
		return nil, fmt.Errorf("failed to export channels: %w", err)
	}
	assets.Channels = channels
	if progress != nil {
		progress("channels", len(channels), len(channels))
	}

	return assets, nil
}

// ExportMemberships exports all memberships (team and channel members)
func (e *Exporter) ExportMemberships(progress ExportProgressCallback) (*Memberships, error) {
	memberships := &Memberships{
		ExportedAt: time.Now().UnixMilli(),
		Version:    "1.0",
	}

	// Export team members
	if progress != nil {
		progress("team_members", 0, 0)
	}
	teamMembers, err := e.client.GetTeamMembers()
	if err != nil {
		return nil, fmt.Errorf("failed to export team members: %w", err)
	}
	memberships.TeamMembers = teamMembers
	if progress != nil {
		progress("team_members", len(teamMembers), len(teamMembers))
	}

	// Export channel members
	if progress != nil {
		progress("channel_members", 0, 0)
	}
	channelMembers, err := e.client.GetChannelMembers()
	if err != nil {
		return nil, fmt.Errorf("failed to export channel members: %w", err)
	}
	memberships.ChannelMembers = channelMembers
	if progress != nil {
		progress("channel_members", len(channelMembers), len(channelMembers))
	}

	return memberships, nil
}

// GetCounts returns the counts of all entities
func (e *Exporter) GetCounts() (users, teams, channels int, err error) {
	users, err = e.client.GetUserCount()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get user count: %w", err)
	}

	teams, err = e.client.GetTeamCount()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get team count: %w", err)
	}

	channels, err = e.client.GetChannelCount()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get channel count: %w", err)
	}

	return users, teams, channels, nil
}

// FilterActiveAssets filters out deleted items from assets
func FilterActiveAssets(assets *Assets) *Assets {
	filtered := &Assets{
		ExportedAt: assets.ExportedAt,
		Version:    assets.Version,
	}

	for _, u := range assets.Users {
		if !u.IsDeleted() {
			filtered.Users = append(filtered.Users, u)
		}
	}

	for _, t := range assets.Teams {
		if !t.IsDeleted() {
			filtered.Teams = append(filtered.Teams, t)
		}
	}

	for _, c := range assets.Channels {
		if !c.IsDeleted() {
			filtered.Channels = append(filtered.Channels, c)
		}
	}

	return filtered
}

// FilterActiveMemberships filters out deleted memberships
func FilterActiveMemberships(memberships *Memberships) *Memberships {
	filtered := &Memberships{
		ExportedAt: memberships.ExportedAt,
		Version:    memberships.Version,
	}

	for _, tm := range memberships.TeamMembers {
		if !tm.IsDeleted() {
			filtered.TeamMembers = append(filtered.TeamMembers, tm)
		}
	}

	// Channel members don't have DeleteAt, copy all
	filtered.ChannelMembers = memberships.ChannelMembers

	return filtered
}




