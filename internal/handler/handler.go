package handler

import (
	"headpat-counter/internal/client"
	"headpat-counter/internal/config"
	"headpat-counter/internal/store"
)

type Handler struct {
	cfg *config.AppConfig
	st  store.Store
	cm  *client.ClientManager
}

func New(cfg *config.AppConfig, cm *client.ClientManager, st store.Store) *Handler {
	return &Handler{cfg: cfg, cm: cm, st: st}
}
