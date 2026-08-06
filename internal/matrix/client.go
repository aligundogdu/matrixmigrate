package matrix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aligundogdu/matrixmigrate/internal/logger"
)

// mediaUploadHTTPTimeout caps how long to wait for Matrix media upload (POST body + response headers).
// The default http.Client timeout (30s) is too low for large files or slow links.
const mediaUploadHTTPTimeout = 45 * time.Minute

// uploadResponseSnippet abbreviates a non-JSON or error upload body for logs (proxies often return HTML/plain text).
func uploadResponseSnippet(b []byte, max int) string {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return "(empty body)"
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	RequestsPerSecond float64 // Max requests per second (0 = no limit)
	MaxRetries        int     // Max retries on 429 error
	RetryBaseDelay    time.Duration // Base delay for exponential backoff
}

// DefaultRateLimitConfig returns default rate limiting settings
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 5.0,               // 5 req/sec
		MaxRetries:        5,                 // 5 retries
		RetryBaseDelay:    2 * time.Second,   // 2 second base delay
	}
}

// Client represents a Matrix API client
type Client struct {
	baseURL    string
	masURL     string // optional: MAS account API base URL for user registration only
	adminToken string
	httpClient *http.Client
	homeserver string
	
	// Application Service support
	asToken    string // AS token for message import with timestamps
	
	// Rate limiting
	lastRequest     time.Time
	rateLimit       time.Duration
	maxRetries      int
	retryBaseDelay  time.Duration
	mu              sync.Mutex
	
	// Transaction ID counter for messages
	txnCounter int64
}

// NewClient creates a new Matrix API client with default rate limiting
func NewClient(baseURL, adminToken, homeserver string) *Client {
	return NewClientWithRateLimit(baseURL, adminToken, homeserver, DefaultRateLimitConfig())
}

// NewClientWithRateLimit creates a new Matrix API client with custom rate limiting
func NewClientWithRateLimit(baseURL, adminToken, homeserver string, rlConfig RateLimitConfig) *Client {
	var rateLimit time.Duration
	if rlConfig.RequestsPerSecond > 0 {
		rateLimit = time.Duration(float64(time.Second) / rlConfig.RequestsPerSecond)
	}
	
	maxRetries := rlConfig.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	
	retryBaseDelay := rlConfig.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = 2 * time.Second
	}
	
	return &Client{
		baseURL:        baseURL,
		masURL:         "",
		adminToken:     adminToken,
		homeserver:     homeserver,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimit:      rateLimit,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
	}
}

// SetHomeserver updates the homeserver domain
func (c *Client) SetHomeserver(homeserver string) {
	c.homeserver = homeserver
}

// SetMASURL sets the Matrix Authentication Service base URL for account creation (POST /api/admin/v1/users).
// Other API calls continue to use the Synapse client base URL.
func (c *Client) SetMASURL(masBaseURL string) {
	c.masURL = strings.TrimSuffix(strings.TrimSpace(masBaseURL), "/")
}

// GetHomeserver returns the current homeserver domain
func (c *Client) GetHomeserver() string {
	return c.homeserver
}

// DetectHomeserver detects the homeserver from the authenticated user ID
// Returns the detected homeserver or error
func (c *Client) DetectHomeserver() (string, error) {
	resp, err := c.WhoAmI()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Parse homeserver from user ID (format: @user:homeserver)
	userID := resp.UserID
	if userID == "" {
		return "", fmt.Errorf("no user_id in response")
	}

	// Find the : separator
	idx := strings.Index(userID, ":")
	if idx == -1 {
		return "", fmt.Errorf("invalid user_id format: %s", userID)
	}

	homeserver := userID[idx+1:]
	if homeserver == "" {
		return "", fmt.Errorf("empty homeserver in user_id: %s", userID)
	}

	logger.Info("Detected homeserver from user ID '%s': %s", userID, homeserver)
	return homeserver, nil
}

// doRequest performs an HTTP request to the Matrix API with rate limiting
func (c *Client) doRequest(method, endpoint string, body interface{}) ([]byte, int, error) {
	return c.doRequestWithRetryToBase(c.baseURL, method, endpoint, body, 0)
}

// doRequestWithRetryToBase performs an HTTP request against apiBase (Synapse or MAS) with rate limiting and 429 retries.
func (c *Client) doRequestWithRetryToBase(apiBase, method, endpoint string, body interface{}, retryCount int) ([]byte, int, error) {
	apiBase = strings.TrimSuffix(strings.TrimSpace(apiBase), "/")
	if apiBase == "" {
		return nil, 0, fmt.Errorf("request base URL is empty")
	}

	// Rate limiting: ensure minimum time between requests
	c.mu.Lock()
	if c.rateLimit > 0 {
		elapsed := time.Since(c.lastRequest)
		if elapsed < c.rateLimit {
			sleepTime := c.rateLimit - elapsed
			time.Sleep(sleepTime)
		}
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	reqURL := strings.TrimSuffix(apiBase, "/") + endpoint
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

	// Handle rate limiting (429) with exponential backoff
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryCount >= c.maxRetries {
			return nil, resp.StatusCode, fmt.Errorf("rate limit exceeded after %d retries", c.maxRetries)
		}
		
		// Try to use Retry-After header if present
		var retryAfter time.Duration
		if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
			// Retry-After can be in seconds (integer) or HTTP-date format
			if seconds, err := strconv.Atoi(retryAfterStr); err == nil {
				retryAfter = time.Duration(seconds) * time.Second
			}
		}
		
		// If no Retry-After header, use exponential backoff
		if retryAfter == 0 {
			// Exponential backoff: base * 2^retryCount (e.g., 2s, 4s, 8s, 16s, 32s)
			retryAfter = c.retryBaseDelay * time.Duration(1<<uint(retryCount))
		}
		
		// Cap the delay at 60 seconds
		if retryAfter > 60*time.Second {
			retryAfter = 60 * time.Second
		}
		
		logger.Warn("Rate limit hit (429), waiting %v before retry %d/%d", retryAfter, retryCount+1, c.maxRetries)
		time.Sleep(retryAfter)
		
		// Retry
		return c.doRequestWithRetryToBase(apiBase, method, endpoint, body, retryCount+1)
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

// CreateUser creates a user via MAS (matrix.api.mas_url) when configured, otherwise Synapse Admin API.
func (c *Client) CreateUser(username string, req *CreateUserRequest) (*UserResponse, error) {
	userID := fmt.Sprintf("@%s:%s", username, c.homeserver)
	if c.masURL != "" {
		return c.createUserViaMAS(username, userID, req)
	}
	return c.createUserViaSynapse(username, userID, req)
}

func (c *Client) createUserViaMAS(username, userID string, req *CreateUserRequest) (*UserResponse, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = fmt.Sprintf("%s@%s", username, c.homeserver)
	}
	masBody := masCreateUserRequest{
		Username: username,
		Password: req.Password,
		Emails:   []masEmailEntry{{Email: email}},
	}
	const endpoint = "/api/admin/v1/users"
	logger.Info("Creating user via MAS: %s %s", username, endpoint)

	body, statusCode, err := c.doRequestWithRetryToBase(c.masURL, "POST", endpoint, masBody, 0)
	if err != nil {
		logger.Error("MAS HTTP request failed for user '%s': %v", username, err)
		return nil, err
	}

	logger.Info("CreateUser (MAS) response for '%s': status=%d", username, statusCode)

	var resp UserResponse
	_ = json.Unmarshal(body, &resp)

	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		resp.UserID = userID
		return &resp, nil
	}

	if statusCode == http.StatusConflict {
		logger.Info("User '%s' already exists on MAS (409), treating as success", username)
		resp.UserID = userID
		return &resp, nil
	}

	errMsg := strings.TrimSpace(resp.Error)
	if errMsg == "" {
		errMsg = strings.TrimSpace(string(body))
	}
	lower := strings.ToLower(errMsg)
	if strings.Contains(lower, "already exists") || strings.Contains(lower, "user already") ||
		strings.Contains(lower, "in use") || resp.Errcode == "M_USER_IN_USE" {
		logger.Info("User '%s' already exists on MAS, treating as success", username)
		resp.UserID = userID
		return &resp, nil
	}

	logger.Error("MAS API error for user '%s': status=%d, body=%s", username, statusCode, string(body))
	return nil, fmt.Errorf("MAS API error (%d): %s", statusCode, errMsg)
}

func (c *Client) createUserViaSynapse(username, userID string, req *CreateUserRequest) (*UserResponse, error) {
	endpoint := fmt.Sprintf("/_synapse/admin/v2/users/%s", url.PathEscape(userID))

	logger.Info("Creating user via Synapse: %s (endpoint: %s)", username, endpoint)

	body, statusCode, err := c.doRequest("PUT", endpoint, req)
	if err != nil {
		logger.Error("HTTP request failed for user '%s': %v", username, err)
		return nil, err
	}

	logger.Info("CreateUser response for '%s': status=%d", username, statusCode)

	var resp UserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.Error("Failed to parse response for user '%s': %v (body: %s)", username, err, string(body))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		// Check if user already exists (some Matrix servers return different codes)
		if resp.Errcode == "M_USER_IN_USE" || strings.Contains(resp.Error, "already exists") {
			logger.Info("User '%s' already exists (status=%d), treating as success", username, statusCode)
			resp.UserID = userID
			return &resp, nil
		}
		logger.Error("API error for user '%s': status=%d, errcode=%s, error=%s", username, statusCode, resp.Errcode, resp.Error)
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
	logger.Info("Checking if user exists: %s", userID)
	user, err := c.GetUser(userID)
	if err != nil {
		logger.Error("UserExists check failed for '%s': %v", username, err)
		return false, err
	}
	exists := user != nil
	logger.Info("User '%s' exists: %v", username, exists)
	return exists, nil
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

// CreateDMRoom creates a direct message room between users.
// name and topic are sent as m.room.name / m.room.topic so clients still show a title after the creator leaves.
func (c *Client) CreateDMRoom(name, topic string, invite []string) (*CreateRoomResponse, error) {
	req := &CreateRoomRequest{
		Name:       name,
		Topic:      topic,
		Visibility: string(VisibilityPrivate),
		Preset:     string(PresetTrustedPrivateChat),
		IsDirect:   true,
		Invite:     invite,
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

// LeaveRoom makes the admin user leave a room
func (c *Client) LeaveRoom(roomID string) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/leave", url.PathEscape(roomID))

	body, statusCode, err := c.doRequest("POST", endpoint, nil)
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

// SetUserPowerLevel sets a single user's power level in a room.
// It reads the current m.room.power_levels state, updates the users map, and writes it back.
// Standard levels: 0 = member, 50 = moderator, 100 = admin.
func (c *Client) SetUserPowerLevel(roomID, userID string, level int) error {
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/state/m.room.power_levels",
		url.PathEscape(roomID))

	// Read current power levels.
	body, statusCode, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to get power levels: %w", err)
	}
	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("failed to get power levels (%d): %s", statusCode, resp.Error)
	}

	var pl map[string]interface{}
	if err := json.Unmarshal(body, &pl); err != nil {
		return fmt.Errorf("failed to parse power levels: %w", err)
	}

	// Ensure the users map exists and set the level.
	users, _ := pl["users"].(map[string]interface{})
	if users == nil {
		users = make(map[string]interface{})
	}
	users[userID] = level
	pl["users"] = users

	// Write back.
	body, statusCode, err = c.doRequest("PUT", endpoint, pl)
	if err != nil {
		return fmt.Errorf("failed to set power levels: %w", err)
	}
	if statusCode != http.StatusOK {
		var resp GenericResponse
		json.Unmarshal(body, &resp)
		return fmt.Errorf("failed to set power levels (%d): %s", statusCode, resp.Error)
	}

	return nil
}

// ForceJoinUser makes a specific user join a room using Synapse Admin API
// This sets the user's membership state to "join" without requiring invitation acceptance
// Required for message import with user impersonation
func (c *Client) ForceJoinUser(roomID, userID string) error {
	endpoint := fmt.Sprintf("/_synapse/admin/v1/rooms/%s/members/%s",
		url.PathEscape(roomID), url.PathEscape(userID))

	req := &MembershipRequest{
		Membership: "join",
	}

	body, statusCode, err := c.doRequest("PUT", endpoint, req)
	if err != nil {
		return err
	}

	// 200 OK is success; 204 No Content can also indicate success
	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		var resp GenericResponse
		if jsonErr := json.Unmarshal(body, &resp); jsonErr == nil {
			return fmt.Errorf("Admin API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
		}
		return fmt.Errorf("Admin API error (%d): %s", statusCode, string(body))
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

// SetASToken sets the Application Service token for message import
func (c *Client) SetASToken(token string) {
	c.asToken = token
}

// HasASToken returns true if an AS token is configured
func (c *Client) HasASToken() bool {
	return c.asToken != ""
}

// getNextTxnID generates a unique transaction ID for messages
func (c *Client) getNextTxnID() string {
	c.mu.Lock()
	c.txnCounter++
	txn := c.txnCounter
	c.mu.Unlock()
	return fmt.Sprintf("mmx_%d_%d", time.Now().UnixMilli(), txn)
}

// SendMessageRequest represents a message to send
type SendMessageRequest struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	FormattedBody string `json:"formatted_body,omitempty"`
	Format        string `json:"format,omitempty"`
}

// SendMessageResponse represents the response from sending a message
type SendMessageResponse struct {
	EventID string `json:"event_id"`
	Errcode string `json:"errcode,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SendMessage sends a message to a room (without timestamp - uses current time)
func (c *Client) SendMessage(roomID, message string) (*SendMessageResponse, error) {
	return c.SendMessageWithTimestamp(roomID, message, "", 0, "")
}

// SendMessageWithTimestamp sends a message to a room with a specific timestamp.
// formattedBody is optional HTML (org.matrix.custom.html); pass "" to omit it.
// timestamp 0 uses current time. senderUserID requires an AS token.
func (c *Client) SendMessageWithTimestamp(roomID, message, formattedBody string, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	txnID := c.getNextTxnID()

	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		url.PathEscape(roomID), url.PathEscape(txnID))

	params := url.Values{}
	if timestamp > 0 && c.asToken != "" {
		params.Set("ts", strconv.FormatInt(timestamp, 10))
	}
	if senderUserID != "" && c.asToken != "" {
		params.Set("user_id", senderUserID)
	}
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req := &SendMessageRequest{
		MsgType: "m.text",
		Body:    message,
	}
	if formattedBody != "" {
		req.Format = "org.matrix.custom.html"
		req.FormattedBody = formattedBody
	}

	token := c.adminToken
	if c.asToken != "" {
		token = c.asToken
	}

	body, statusCode, err := c.doRequestWithToken("PUT", endpoint, req, token)
	if err != nil {
		return nil, err
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// SendReplyWithTimestamp sends a reply to a message with a specific timestamp.
// formattedBody is optional HTML (org.matrix.custom.html); pass "" to omit it.
func (c *Client) SendReplyWithTimestamp(roomID, message, formattedBody string, replyToEventID string, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	txnID := c.getNextTxnID()

	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		url.PathEscape(roomID), url.PathEscape(txnID))

	params := url.Values{}
	if timestamp > 0 && c.asToken != "" {
		params.Set("ts", strconv.FormatInt(timestamp, 10))
	}
	if senderUserID != "" && c.asToken != "" {
		params.Set("user_id", senderUserID)
	}
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	content := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
		"m.relates_to": map[string]interface{}{
			"m.in_reply_to": map[string]string{
				"event_id": replyToEventID,
			},
		},
	}
	if formattedBody != "" {
		content["format"] = "org.matrix.custom.html"
		content["formatted_body"] = formattedBody
	}

	token := c.adminToken
	if c.asToken != "" {
		token = c.asToken
	}

	body, statusCode, err := c.doRequestWithToken("PUT", endpoint, content, token)
	if err != nil {
		return nil, err
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// SendThreadMessageWithTimestamp sends a thread message (MSC3440) with a specific timestamp.
// threadRootEventID is the root event of the thread, lastEventInThread is the latest event in that thread.
func (c *Client) SendThreadMessageWithTimestamp(
    roomID, message, formattedBody, threadRootEventID, lastEventInThread string,
    timestamp int64, senderUserID string,
) (*SendMessageResponse, error) {
    txnID := c.getNextTxnID()

    endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
        url.PathEscape(roomID), url.PathEscape(txnID))

    params := url.Values{}
    if timestamp > 0 && c.asToken != "" {
        params.Set("ts", strconv.FormatInt(timestamp, 10))
    }
    if senderUserID != "" && c.asToken != "" {
        params.Set("user_id", senderUserID)
    }
    if len(params) > 0 {
        endpoint += "?" + params.Encode()
    }

    content := map[string]interface{}{
        "msgtype": "m.text",
        "body":    message,
        "m.relates_to": map[string]interface{}{
            "rel_type": "m.thread",
            "event_id": threadRootEventID,
            "is_falling_back": false,
        },
        "m.thread": map[string]interface{}{
            "latest_event_id": lastEventInThread,
        },
    }
    if formattedBody != "" {
        content["format"] = "org.matrix.custom.html"
        content["formatted_body"] = formattedBody
    }

    token := c.adminToken
    if c.asToken != "" {
        token = c.asToken
    }

    body, statusCode, err := c.doRequestWithToken("PUT", endpoint, content, token)
    if err != nil {
        return nil, err
    }

    var resp SendMessageResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    if statusCode != http.StatusOK {
        return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
    }

    return &resp, nil
}

// doRequestWithToken performs an HTTP request with a specific token
func (c *Client) doRequestWithToken(method, endpoint string, body interface{}, token string) ([]byte, int, error) {
	return c.doRequestWithTokenAndRetry(method, endpoint, body, token, 0)
}

// doRequestWithTokenAndRetry performs an HTTP request with retry logic
func (c *Client) doRequestWithTokenAndRetry(method, endpoint string, body interface{}, token string, retryCount int) ([]byte, int, error) {
	// Rate limiting
	c.mu.Lock()
	if c.rateLimit > 0 {
		elapsed := time.Since(c.lastRequest)
		if elapsed < c.rateLimit {
			sleepTime := c.rateLimit - elapsed
			time.Sleep(sleepTime)
		}
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

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

	req.Header.Set("Authorization", "Bearer "+token)
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

	// Handle rate limiting (429) with exponential backoff
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryCount >= c.maxRetries {
			return nil, resp.StatusCode, fmt.Errorf("rate limit exceeded after %d retries", c.maxRetries)
		}
		
		var retryAfter time.Duration
		if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
			if seconds, err := strconv.Atoi(retryAfterStr); err == nil {
				retryAfter = time.Duration(seconds) * time.Second
			}
		}
		
		if retryAfter == 0 {
			retryAfter = c.retryBaseDelay * time.Duration(1<<uint(retryCount))
		}
		
		if retryAfter > 60*time.Second {
			retryAfter = 60 * time.Second
		}
		
		logger.Warn("Rate limit hit (429), waiting %v before retry %d/%d", retryAfter, retryCount+1, c.maxRetries)
		time.Sleep(retryAfter)
		
		return c.doRequestWithTokenAndRetry(method, endpoint, body, token, retryCount+1)
	}

	return respBody, resp.StatusCode, nil
}

// UploadMediaResponse represents the response from media upload
type UploadMediaResponse struct {
	ContentURI string `json:"content_uri"` // mxc://server/media_id
	Errcode    string `json:"errcode,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UploadMedia uploads a file to Matrix media repository
// Returns the mxc:// URI for the uploaded file
func (c *Client) UploadMedia(data []byte, filename, contentType string) (*UploadMediaResponse, error) {
	endpoint := fmt.Sprintf("/_matrix/media/v3/upload?filename=%s", url.QueryEscape(filename))
	
	// Rate limiting
	c.mu.Lock()
	if c.rateLimit > 0 {
		elapsed := time.Since(c.lastRequest)
		if elapsed < c.rateLimit {
			time.Sleep(c.rateLimit - elapsed)
		}
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()
	
	reqURL := c.baseURL + endpoint
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}
	
	token := c.adminToken
	if c.asToken != "" {
		token = c.asToken
	}
	
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)

	uploadClient := &http.Client{Timeout: mediaUploadHTTPTimeout}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp UploadMediaResponse
		if json.Unmarshal(body, &errResp) == nil && (errResp.Errcode != "" || errResp.Error != "") {
			return nil, fmt.Errorf("upload failed (%d): %s - %s", resp.StatusCode, errResp.Errcode, errResp.Error)
		}
		return nil, fmt.Errorf("upload failed (HTTP %d), non-JSON body: %s", resp.StatusCode, uploadResponseSnippet(body, 400))
	}

	body = bytes.TrimSpace(body)
	var result UploadMediaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("upload HTTP %d but invalid JSON: %w; body: %s", resp.StatusCode, err, uploadResponseSnippet(body, 400))
	}
	if strings.TrimSpace(result.ContentURI) == "" {
		return nil, fmt.Errorf("upload missing content_uri in JSON: %s", uploadResponseSnippet(body, 400))
	}

	return &result, nil
}

// FileMessageContent represents a file message content
type FileMessageContent struct {
	MsgType  string         `json:"msgtype"`           // m.file, m.image, m.video, m.audio
	Body     string         `json:"body"`              // Filename as fallback
	URL      string         `json:"url,omitempty"`     // mxc:// URI (for uploaded files)
	Filename string         `json:"filename,omitempty"`
	Info     *FileInfo      `json:"info,omitempty"`
}

// FileInfo contains metadata about the file
type FileInfo struct {
	MimeType      string `json:"mimetype,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Width         int    `json:"w,omitempty"`          // For images/videos
	Height        int    `json:"h,omitempty"`          // For images/videos
	Duration      int    `json:"duration,omitempty"`   // For audio/video in ms
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`
	ThumbnailInfo *ThumbnailInfo `json:"thumbnail_info,omitempty"`
}

// ThumbnailInfo contains metadata about thumbnail
type ThumbnailInfo struct {
	MimeType string `json:"mimetype,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Width    int    `json:"w,omitempty"`
	Height   int    `json:"h,omitempty"`
}

// SendFileMessage sends a file message to a room
func (c *Client) SendFileMessage(roomID string, content *FileMessageContent, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	txnID := c.getNextTxnID()

	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		url.PathEscape(roomID), url.PathEscape(txnID))

	params := url.Values{}
	if timestamp > 0 && c.asToken != "" {
		params.Set("ts", strconv.FormatInt(timestamp, 10))
	}
	if senderUserID != "" && c.asToken != "" {
		params.Set("user_id", senderUserID)
	}
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	token := c.adminToken
	if c.asToken != "" {
		token = c.asToken
	}

	body, statusCode, err := c.doRequestWithToken("PUT", endpoint, content, token)
	if err != nil {
		return nil, err
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s - %s", statusCode, resp.Errcode, resp.Error)
	}

	return &resp, nil
}

// SendFileLink sends a message with a file link (external URL)
// Note: Matrix doesn't support external URLs directly in file messages,
// so we send as a text message with a markdown link
func (c *Client) SendFileLink(roomID, filename, fileURL, mimeType string, fileSize int64, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	// Determine emoji based on file type
	emoji := "📎"
	if strings.HasPrefix(mimeType, "image/") {
		emoji = "🖼️"
	} else if strings.HasPrefix(mimeType, "video/") {
		emoji = "🎬"
	} else if strings.HasPrefix(mimeType, "audio/") {
		emoji = "🎵"
	}
	
	message := fmt.Sprintf("%s [%s](%s)", emoji, filename, fileURL)

	return c.SendMessageWithTimestamp(roomID, message, "", timestamp, senderUserID)
}

// SendUploadedFile sends a file that was already uploaded to Matrix
func (c *Client) SendUploadedFile(roomID, mxcURI, filename, mimeType string, fileSize int64, width, height int, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	msgType := "m.file"
	if strings.HasPrefix(mimeType, "image/") {
		msgType = "m.image"
	} else if strings.HasPrefix(mimeType, "video/") {
		msgType = "m.video"
	} else if strings.HasPrefix(mimeType, "audio/") {
		msgType = "m.audio"
	}
	
	content := &FileMessageContent{
		MsgType:  msgType,
		Body:     filename,
		URL:      mxcURI,
		Filename: filename,
		Info: &FileInfo{
			MimeType: mimeType,
			Size:     fileSize,
		},
	}
	
	if width > 0 && height > 0 {
		content.Info.Width = width
		content.Info.Height = height
	}
	
	return c.SendFileMessage(roomID, content, timestamp, senderUserID)
}
