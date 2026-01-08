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
	return c.doRequestWithRetry(method, endpoint, body, 0)
}

// doRequestWithRetry performs an HTTP request with retry logic for rate limiting
func (c *Client) doRequestWithRetry(method, endpoint string, body interface{}, retryCount int) ([]byte, int, error) {
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
		return c.doRequestWithRetry(method, endpoint, body, retryCount+1)
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

	logger.Info("Creating user: %s (endpoint: %s)", username, endpoint)

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
	return c.SendMessageWithTimestamp(roomID, message, 0, "")
}

// SendMessageWithTimestamp sends a message to a room with a specific timestamp
// This requires an Application Service token to be set
// If timestamp is 0, uses current time
// If senderUserID is provided, the message will appear as sent by that user (requires AS)
func (c *Client) SendMessageWithTimestamp(roomID, message string, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	txnID := c.getNextTxnID()
	
	// Build endpoint
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		url.PathEscape(roomID), url.PathEscape(txnID))
	
	// Add query parameters
	params := url.Values{}
	
	// Add timestamp if AS token is available and timestamp is provided
	if timestamp > 0 && c.asToken != "" {
		params.Set("ts", strconv.FormatInt(timestamp, 10))
	}
	
	// Add user_id parameter for AS to send on behalf of user
	if senderUserID != "" && c.asToken != "" {
		params.Set("user_id", senderUserID)
	}
	
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	
	// Create message content
	req := &SendMessageRequest{
		MsgType: "m.text",
		Body:    message,
	}
	
	// Use AS token if available, otherwise use admin token
	token := c.adminToken
	if c.asToken != "" {
		token = c.asToken
	}
	
	// Make request
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

// SendReplyWithTimestamp sends a reply to a message with a specific timestamp
func (c *Client) SendReplyWithTimestamp(roomID, message string, replyToEventID string, timestamp int64, senderUserID string) (*SendMessageResponse, error) {
	txnID := c.getNextTxnID()
	
	// Build endpoint
	endpoint := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		url.PathEscape(roomID), url.PathEscape(txnID))
	
	// Add query parameters
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
	
	// Create reply content with relation
	content := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
		"m.relates_to": map[string]interface{}{
			"m.in_reply_to": map[string]string{
				"event_id": replyToEventID,
			},
		},
	}
	
	// Use AS token if available
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
