package config

import (
	"log"
	"os"
	"time"
)

type AppConfig struct {
	AppClientId     string
	BaseURL         string
	BroadcasterId   string
	CookieName      string
	Environment     string
	RewardId        string
	PollerFrequency time.Duration
	PollerTimeout   time.Duration
}

const (
	defaultPollerFrequency time.Duration = 2 * time.Second
	defaultPollerTimeout   time.Duration = 10 * time.Minute
)

func NewConfigFromEnv() AppConfig {
	cfg := AppConfig{
		AppClientId:   os.Getenv("APP_CLIENT_ID"),
		BaseURL:       os.Getenv("BASE_URL"),
		BroadcasterId: os.Getenv("BROADCASTER_USER_ID"),
		CookieName:    os.Getenv("COOKIE_NAME"),
		Environment:   os.Getenv("MODE"),
		RewardId:      os.Getenv("REWARD_ID"),
	}

	if cfg.AppClientId == "" {
		log.Fatal("Missing client_id\n")
	}

	if cfg.BaseURL == "" {
		log.Fatal("Missing base URL\n")
	}

	if cfg.CookieName == "" {
		log.Fatal("Missing cookie name\n")
	}

	if !cfg.IsDev() {
		if cfg.BroadcasterId == "" {
			log.Fatal("Missing broadcaster_user_id\n")
		}

		if cfg.RewardId == "" {
			log.Fatal("Missing reward_id\n")
		}
	}

	pollerFrequency, err := time.ParseDuration(os.Getenv("POLLER_FREQUENCY"))
	if err != nil {
		log.Fatalf("Unable to parse poller frequency: %v\n", err)
	}

	cfg.PollerFrequency = pollerFrequency
	if cfg.PollerFrequency == 0 {
		cfg.PollerFrequency = defaultPollerFrequency
	}

	pollerTimeout, err := time.ParseDuration(os.Getenv("POLLER_TIMEOUT"))
	if err != nil {
		log.Fatalf("Unable to parse poller timeout: %v\n", err)
	}

	cfg.PollerTimeout = pollerTimeout
	if cfg.PollerTimeout == 0 {
		cfg.PollerTimeout = defaultPollerTimeout
	}

	return cfg
}

func (cfg AppConfig) IsDev() bool {
	return cfg.Environment == "DEV"
}
