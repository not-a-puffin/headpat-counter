package request

import (
	"encoding/json"
	"fmt"
	"headpat-counter/internal/config"
	"io"
	"log"
	"net/http"
)

type TwitchUser struct {
	Id              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	Type            string `json:"type"`
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	ProfileImageUrl string `json:"profile_image_url"`
	OfflineImageUrl string `json:"offline_image_url"`
	ViewCount       int    `json:"view_count"`
	Email           string `json:"email"`
	CreatedAt       string `json:"created_at"`
}

type GetTwitchUserResult struct {
	Data []TwitchUser `json:"data"`
}

func GetTwitchUser(cfg *config.AppConfig, accessToken string) (*TwitchUser, error) {
	helixURL := "https://api.twitch.tv/helix/users"

	log.Println("Getting Twitch user")

	req, err := http.NewRequest("GET", helixURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Client-Id", cfg.AppClientId)

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

	var result GetTwitchUserResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no user found")
	}

	if len(result.Data) > 1 {
		return nil, fmt.Errorf("multiple users found")
	}

	return &result.Data[0], nil
}
