package matrix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LoginRequest represents a Matrix login request
type LoginRequest struct {
	Type       string `json:"type"`
	User       string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	InitialDeviceDisplayName string `json:"initial_device_display_name,omitempty"`
}

// LoginResponse represents a Matrix login response
type LoginResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
	HomeServer  string `json:"home_server"`
	Errcode     string `json:"errcode,omitempty"`
	Error       string `json:"error,omitempty"`
}

// LoginFlowsResponse represents the available login flows
type LoginFlowsResponse struct {
	Flows []struct {
		Type string `json:"type"`
	} `json:"flows"`
}

// Login authenticates with Matrix and returns an access token
func Login(baseURL, username, password string) (*LoginResponse, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare login request
	loginReq := &LoginRequest{
		Type:       "m.login.password",
		User:       username,
		Password:   password,
		DeviceID:   "matrixmigrate",
		InitialDeviceDisplayName: "MatrixMigrate CLI",
	}

	reqBody, err := json.Marshal(loginReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login request: %w", err)
	}

	// Send login request
	loginURL := baseURL + "/_matrix/client/v3/login"
	resp, err := httpClient.Post(loginURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %s - %s", loginResp.Errcode, loginResp.Error)
	}

	if loginResp.AccessToken == "" {
		return nil, fmt.Errorf("login succeeded but no access token received")
	}

	return &loginResp, nil
}

// CheckLoginFlows checks available login methods
func CheckLoginFlows(baseURL string) (*LoginFlowsResponse, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Get(baseURL + "/_matrix/client/v3/login")
	if err != nil {
		return nil, fmt.Errorf("failed to get login flows: %w", err)
	}
	defer resp.Body.Close()

	var flows LoginFlowsResponse
	if err := json.NewDecoder(resp.Body).Decode(&flows); err != nil {
		return nil, fmt.Errorf("failed to parse login flows: %w", err)
	}

	return &flows, nil
}

// SupportsPasswordLogin checks if the server supports password login
func SupportsPasswordLogin(baseURL string) (bool, error) {
	flows, err := CheckLoginFlows(baseURL)
	if err != nil {
		return false, err
	}

	for _, flow := range flows.Flows {
		if flow.Type == "m.login.password" {
			return true, nil
		}
	}

	return false, nil
}

// Logout invalidates the access token
func Logout(baseURL, accessToken string) error {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", baseURL+"/_matrix/client/v3/logout", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logout failed with status: %d", resp.StatusCode)
	}

	return nil
}

