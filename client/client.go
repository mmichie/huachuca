package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	accessToken string
	csrfToken   string
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Set tokens
func (c *Client) SetAccessToken(token string) { c.accessToken = token }
func (c *Client) SetCSRFToken(token string)   { c.csrfToken = token }

// TokenResponse represents the auth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// User represents a Huachuca user
type User struct {
	ID             string          `json:"id"`
	Email          string          `json:"email"`
	Name           string          `json:"name"`
	OrganizationID string          `json:"organization_id"`
	Role           string          `json:"role"`
	Permissions    map[string]bool `json:"permissions"`
}

// GetGoogleAuthURL returns the Google OAuth URL
func (c *Client) GetGoogleAuthURL() string {
	return fmt.Sprintf("%s/auth/login/google", c.baseURL)
}

// RefreshToken refreshes an access token
func (c *Client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	reqBody, err := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/auth/refresh", c.baseURL),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token request failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// GetCSRFToken gets a new CSRF token
func (c *Client) GetCSRFToken() (string, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/csrf/token", c.baseURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CSRF token request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

// GetUser gets the current user's information
func (c *Client) GetUser() (*User, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/user", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get user failed with status %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}
