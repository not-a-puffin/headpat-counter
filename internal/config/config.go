package config

import (
	"log"
	"os"
)

type AppConfig struct {
	AppClientId   string
	BaseURL       string
	BroadcasterId string
	Environment   string
	RewardId      string
}

func NewConfigFromEnv() AppConfig {
	cfg := AppConfig{
		AppClientId:   os.Getenv("APP_CLIENT_ID"),
		BaseURL:       os.Getenv("BASE_URL"),
		BroadcasterId: os.Getenv("BROADCASTER_USER_ID"),
		Environment:   os.Getenv("MODE"),
		RewardId:      os.Getenv("REWARD_ID"),
	}

	if cfg.AppClientId == "" {
		log.Fatal("Missing client_id\n")
	}

	if cfg.BaseURL == "" {
		log.Fatal("Missing base URL\n")
	}

	if !cfg.IsDev() {
		if cfg.BroadcasterId == "" {
			log.Fatal("Missing broadcaster_user_id\n")
		}

		if cfg.RewardId == "" {
			log.Fatal("Missing reward_id\n")
		}
	}

	return cfg
}

func (cfg AppConfig) IsDev() bool {
	return cfg.Environment == "DEV"
}
