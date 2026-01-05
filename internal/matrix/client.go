package matrix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents a Matrix API client
type Client struct {
	baseURL    string
	adminToken string
	httpClient *http.Client
	homeserver string
}

// NewClient creates a new Matrix API client
func NewClient(baseURL, adminToken, homeserver string) *Client {
	return &Client{
		baseURL:    baseURL,
		adminToken: adminToken,
		homeserver: homeserver,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request to the Matrix API
func (c *Client) doRequest(method, endpoint string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	reqURL := c.baseURL + endpoint
	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// WhoAmI returns the current user ID for the admin token
func (c *Client) WhoAmI() (*WhoAmIResponse, error) {
	body, statusCode, err := c.doRequest("GET", "/_matrix/client/v3/account/whoami", nil)
	if err != nil {
		return nil, err
	}

	var resp WhoAmIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// TestConnection tests the API connection
func (c *Client) TestConnection() error {
	_, err := c.WhoAmI()
	return err
}

// CreateUser creates or updates a user via the Admin API
func (c *Client) CreateUser(username string, req *CreateUserRequest) (*UserResponse, error) {
	userID := fmt.Sprintf("@%s:%s", username, c.homeserver)
	endpoint := fmt.Sprintf("/_synapse/admin/v2/users/%s", url.PathEscape(userID))

	body, statusCode, err := c.doRequest("PUT", endpoint, req)
	if err != nil {
		return nil, err
	}

	var resp UserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	resp.UserID = userID
	return &resp, nil
}

// GetUser gets user info via the Admin API
func (c *Client) GetUser(userID string) (*UserResponse, error) {
	endpoint := fmt.Sprintf("/_synapse/admin/v2/users/%s", url.PathEscape(userID))

	body, statusCode, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp UserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, nil // User doesn't exist
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// UserExists checks if a user exists
func (c *Client) UserExists(username string) (bool, error) {
	userID := fmt.Sprintf("@%s:%s", username, c.homeserver)
	user, err := c.GetUser(userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

// CreateRoom creates a new room
func (c *Client) CreateRoom(req *CreateRoomRequest) (*CreateRoomResponse, error) {
	body, statusCode, err := c.doRequest("POST", "/_matrix/client/v3/createRoom", req)
	if err != nil {
		return nil, err
	}

	var resp CreateRoomResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// CreateSpace creates a new space (a room with m.space type)
func (c *Client) CreateSpace(name, topic string, public bool) (*CreateRoomResponse, error) {
	visibility := VisibilityPrivate
	preset := PresetPrivateChat
	if public {
		visibility = VisibilityPublic
		preset = PresetPublicChat
	}

	req := &CreateRoomRequest{
		Name:       name,
		Topic:      topic,
		Visibility: string(visibility),
		Preset:     string(preset),
		CreationContent: map[string]interface{}{
			"type": SpaceType,
		},
	}

	return c.CreateRoom(req)
}

// CreateRegularRoom creates a regular room (not a space)
func (c *Client) CreateRegularRoom(name, topic string, public bool) (*CreateRoomResponse, error) {
	visibility := VisibilityPrivate
	preset := PresetPrivateChat
	if public {
		visibility = VisibilityPublic
		preset = PresetPublicChat
	}

	req := &CreateRoomRequest{
		Name:       name,
		Topic:      topic,
		Visibility: string(visibility),
		Preset:     string(preset),
	}

	return c.CreateRoom(req)
}

// InviteUser invites a user to a room
func (c *Client) InviteUser(roomID, userID string) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/invite", url.PathEscape(roomID))

	req := &InviteRequest{
		UserID: userID,
	}

	body, statusCode, err := c.doRequest("POST", endpoint, req)
	if err != nil {
		return err
	}

	if statusCode == http.StatusForbidden {
		// User might already be in the room
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		if resp.Errcode == "M_FORBIDDEN" {
			return nil // Already a member, not an error
		}
	}

	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return nil
}

// JoinRoom makes the admin user join a room (needed before inviting others in some cases)
func (c *Client) JoinRoom(roomID string) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/join", url.PathEscape(roomID))

	body, statusCode, err := c.doRequest("POST", endpoint, &JoinRequest{})
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return nil
}

// AddRoomToSpace adds a room as a child of a space
func (c *Client) AddRoomToSpace(spaceID, roomID string, suggested bool) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/state/%s/%s",
		url.PathEscape(spaceID),
		EventTypeSpaceChild,
		url.PathEscape(roomID))

	content := &SpaceChildContent{
		Via:       []string{c.homeserver},
		Suggested: suggested,
	}

	body, statusCode, err := c.doRequest("PUT", endpoint, content)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return nil
}

// SetRoomParent sets the parent space for a room
func (c *Client) SetRoomParent(roomID, spaceID string, canonical bool) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/state/%s/%s",
		url.PathEscape(roomID),
		EventTypeSpaceParent,
		url.PathEscape(spaceID))

	content := &SpaceParentContent{
		Via:       []string{c.homeserver},
		Canonical: canonical,
	}

	body, statusCode, err := c.doRequest("PUT", endpoint, content)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return nil
}

// FormatUserID formats a username as a full Matrix user ID
func (c *Client) FormatUserID(username string) string {
	return fmt.Sprintf("@%s:%s", username, c.homeserver)
}

// GetHomeserver returns the homeserver domain
func (c *Client) GetHomeserver() string {
	return c.homeserver
}

