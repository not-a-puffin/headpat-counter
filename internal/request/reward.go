package request

import (
	"encoding/json"
	"fmt"
	"headpat-counter/internal/config"
	"io"
	"net/http"
	"net/url"
)

type CustomReward struct {
	BroadcasterId                    string          `json:"broadcaster_id"`
	BroadcasterLogin                 string          `json:"broadcaster_login"`
	BroadcasterName                  string          `json:"broadcaster_name"`
	Id                               string          `json:"id"`
	Title                            string          `json:"title"`
	Prompt                           string          `json:"prompt"`
	Cost                             int             `json:"cost"`
	Image                            json.RawMessage `json:"image"`
	DefaultImage                     json.RawMessage `json:"default_image"`
	BackgroundColor                  string          `json:"background_color"`
	IsEnabled                        bool            `json:"is_enabled"`
	IsUserInputRequired              bool            `json:"is_user_input_required"`
	MaxPerStreamSetting              json.RawMessage `json:"max_per_stream_setting"`
	MaxPerUserSetting                json.RawMessage `json:"max_per_user_per_stream_setting"`
	GlobalCooldownSetting            json.RawMessage `json:"global_cooldown_setting"`
	IsPaused                         bool            `json:"is_paused"`
	IsInStock                        bool            `json:"is_in_stock"`
	SkipRequestQueue                 bool            `json:"should_redemptions_skip_request_queue"`
	RedemptionsRedeemedCurrentStream *int            `json:"redemptions_redeemed_current_stream"`
	CooldownExpiresAt                *string         `json:"cooldown_expires_at"`
}

type CustomRewardResult struct {
	Data []CustomReward `json:"data"`
}

func GetHeadpatReward(cfg *config.AppConfig, accessToken string) (*CustomReward, error) {
	helixURL := "https://api.twitch.tv/helix/channel_points/custom_rewards"

	params := url.Values{}
	params.Add("broadcaster_id", cfg.BroadcasterId)
	params.Add("id", cfg.RewardId)
	url := helixURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", url, nil)
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

	var result CustomRewardResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no custom reward found")
	}

	if len(result.Data) > 1 {
		return nil, fmt.Errorf("multiple custom rewards found")
	}

	return &result.Data[0], nil
}
