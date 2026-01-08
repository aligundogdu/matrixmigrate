package mattermost

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// Client represents a Mattermost database client
type Client struct {
	db *sql.DB
}

// NewClient creates a new Mattermost database client
func NewClient(dsn string) (*Client, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{db: db}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}

// Ping tests the database connection
func (c *Client) Ping() error {
	return c.db.Ping()
}

// GetUsers retrieves all users from the database
func (c *Client) GetUsers() ([]User, error) {
	query := `
		SELECT 
			id, username, email, 
			COALESCE(firstname, '') as firstname, 
			COALESCE(lastname, '') as lastname,
			COALESCE(nickname, '') as nickname,
			COALESCE(position, '') as position,
			COALESCE(locale, 'en') as locale,
			COALESCE(timezone::text, '{}') as timezone,
			createat, updateat, deleteat,
			COALESCE(roles, '') as roles
		FROM users
		ORDER BY createat ASC
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(
			&u.ID, &u.Username, &u.Email,
			&u.FirstName, &u.LastName, &u.Nickname,
			&u.Position, &u.Locale, &u.Timezone,
			&u.CreateAt, &u.UpdateAt, &u.DeleteAt,
			&u.Roles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// GetTeams retrieves all teams from the database
func (c *Client) GetTeams() ([]Team, error) {
	query := `
		SELECT 
			id, name, displayname, 
			COALESCE(description, '') as description,
			COALESCE(email, '') as email,
			type,
			COALESCE(companyname, '') as companyname,
			COALESCE(alloweddomains, '') as alloweddomains,
			COALESCE(inviteid, '') as inviteid,
			allowopeninvite,
			createat, updateat, deleteat
		FROM teams
		ORDER BY createat ASC
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var t Team
		err := rows.Scan(
			&t.ID, &t.Name, &t.DisplayName,
			&t.Description, &t.Email, &t.Type,
			&t.CompanyName, &t.AllowedDomains, &t.InviteID,
			&t.AllowOpenInvite,
			&t.CreateAt, &t.UpdateAt, &t.DeleteAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating teams: %w", err)
	}

	return teams, nil
}

// GetChannels retrieves all channels from the database
func (c *Client) GetChannels() ([]Channel, error) {
	query := `
		SELECT 
			id, 
			COALESCE(teamid, '') as teamid, 
			name, displayname,
			COALESCE(header, '') as header,
			COALESCE(purpose, '') as purpose,
			type,
			createat, updateat, deleteat,
			COALESCE(creatorid, '') as creatorid,
			COALESCE(totalmsgcount, 0) as totalmsgcount
		FROM channels
		WHERE type IN ('O', 'P', 'G')
		ORDER BY createat ASC
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		err := rows.Scan(
			&ch.ID, &ch.TeamID, &ch.Name, &ch.DisplayName,
			&ch.Header, &ch.Purpose, &ch.Type,
			&ch.CreateAt, &ch.UpdateAt, &ch.DeleteAt,
			&ch.CreatorID, &ch.TotalMsgCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// GetTeamMembers retrieves all team memberships from the database
func (c *Client) GetTeamMembers() ([]TeamMember, error) {
	query := `
		SELECT 
			teamid, userid, 
			COALESCE(roles, '') as roles,
			deleteat
		FROM teammembers
		ORDER BY teamid, userid
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var tm TeamMember
		err := rows.Scan(&tm.TeamID, &tm.UserID, &tm.Roles, &tm.DeleteAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		members = append(members, tm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating team members: %w", err)
	}

	return members, nil
}

// GetChannelMembers retrieves all channel memberships from the database
func (c *Client) GetChannelMembers() ([]ChannelMember, error) {
	query := `
		SELECT 
			channelid, userid, 
			COALESCE(roles, '') as roles,
			COALESCE(notifyprops::text, '{}') as notifyprops,
			COALESCE(lastviewedat, 0) as lastviewedat,
			COALESCE(msgcount, 0) as msgcount
		FROM channelmembers
		ORDER BY channelid, userid
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel members: %w", err)
	}
	defer rows.Close()

	var members []ChannelMember
	for rows.Next() {
		var cm ChannelMember
		err := rows.Scan(
			&cm.ChannelID, &cm.UserID, &cm.Roles,
			&cm.NotifyProps, &cm.LastViewedAt, &cm.MsgCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel member: %w", err)
		}
		members = append(members, cm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel members: %w", err)
	}

	return members, nil
}

// GetUserCount returns the total number of users
func (c *Client) GetUserCount() (int, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetTeamCount returns the total number of teams
func (c *Client) GetTeamCount() (int, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM teams").Scan(&count)
	return count, err
}

// GetChannelCount returns the total number of channels (public, private, and group)
func (c *Client) GetChannelCount() (int, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM channels WHERE type IN ('O', 'P', 'G')").Scan(&count)
	return count, err
}




