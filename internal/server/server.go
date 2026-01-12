package server

import (
	"fmt"
	"headpat-counter/internal/client"
	"headpat-counter/internal/config"
	"headpat-counter/internal/handler"
	"headpat-counter/internal/store"
	"net/http"
)

func New(port string) *http.Server {
	cfg := config.NewConfigFromEnv()
	cm := client.NewClientManager()
	st := store.NewRedisStore()
	mux := http.NewServeMux()

	if cfg.IsDev() {
		fmt.Println("Starting server in development mode")
	}

	fs := http.FileServer(http.Dir("client"))
	mux.Handle("/overlay/", fs)
	mux.Handle("/favicon.ico", fs)
	mux.Handle("/favicon.svg", fs)
	mux.Handle("/favicon-96x96.png", fs)

	authHandler := handler.NewAuthHandler(&cfg, st)
	mux.HandleFunc("/auth/connect", authHandler.ConnectToTwitch)
	mux.HandleFunc("/auth/callback", authHandler.Callback)

	eventsubHandler := handler.NewEventsubHandler(&cfg, &cm, st)
	mux.HandleFunc("POST /notification", eventsubHandler.Callback)

	headpatHandler := handler.NewHeadpatHandler(&cfg, &cm, st)
	mux.HandleFunc("/headpat/count", headpatHandler.GetCount)
	mux.HandleFunc("/headpat/leaderboard", headpatHandler.GetLeaderboard)
	mux.HandleFunc("/headpat/leaderboard/{user}", headpatHandler.GetLeaderboardRankForUser)
	mux.Handle("POST /headpat/fulfill", authHandler.Middleware(http.HandlerFunc(headpatHandler.Fulfill)))
	mux.HandleFunc("/headpat/events", headpatHandler.Events)

	mux.Handle("/auth/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler.HasAuth(r) {
			http.Redirect(w, r, "/control-panel/", http.StatusSeeOther)
		} else {
			http.ServeFile(w, r, "client/auth")
		}
	})))

	mux.Handle("/control-panel/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler.HasAuth(r) {
			http.ServeFile(w, r, "client/control-panel")
		} else {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
		}
	})))

	mux.Handle("/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if handler.HasAuth(r) {
			http.Redirect(w, r, "/control-panel/", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
		}
	})))

	return &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
}
