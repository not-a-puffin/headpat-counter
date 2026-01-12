package request

import (
	"encoding/json"
	"fmt"
	"headpat-counter/internal/config"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type OauthTokenResult struct {
	AccessToken  string          `json:"access_token"`
	ExpiresIn    int             `json:"expires_in"`
	RefreshToken string          `json:"refresh_token"`
	Scope        json.RawMessage `json:"scope"`
	TokenType    string          `json:"token_type"`
}

const oauthURL string = "https://id.twitch.tv/oauth2/token"

// Request an OAuth user access token using the authorization code grant flow
// https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#authorization-code-grant-flow
func GetUserAccessToken(cfg *config.AppConfig, code string) (*OauthTokenResult, error) {
	log.Println("Getting new user access token")

	secret := os.Getenv("APP_CLIENT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("invalid client secret")
	}

	params := url.Values{}
	params.Add("client_id", cfg.AppClientId)
	params.Add("client_secret", secret)
	params.Add("code", code)
	params.Add("grant_type", "authorization_code")
	params.Add("redirect_uri", cfg.BaseURL+"/auth/callback")
	payload := params.Encode()

	req, err := http.NewRequest("POST", oauthURL, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result OauthTokenResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return &result, nil
}

// Request an OAuth user access token using a refresh token
// https://dev.twitch.tv/docs/authentication/refresh-tokens/
func RefreshUserAccessToken(cfg *config.AppConfig, refreshToken string) (*OauthTokenResult, error) {
	log.Println("Refreshing user access token")

	secret := os.Getenv("APP_CLIENT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("invalid client secret")
	}

	params := url.Values{}
	params.Add("client_id", cfg.AppClientId)
	params.Add("client_secret", secret)
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", refreshToken)
	payload := params.Encode()

	req, err := http.NewRequest("POST", oauthURL, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result OauthTokenResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return &result, nil
}
