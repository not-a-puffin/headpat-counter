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

	hdl := handler.New(&cfg, &cm, st)
	mux.HandleFunc("/auth/connect", hdl.ConnectToTwitch)
	mux.HandleFunc("/auth/callback", hdl.AuthCallback)

	mux.HandleFunc("POST /notification", hdl.EventsubCallback)

	mux.HandleFunc("/headpat/count", hdl.GetCount)
	mux.HandleFunc("/headpat/events", hdl.Events)
	mux.HandleFunc("/headpat/leaderboard", hdl.GetLeaderboard)
	mux.HandleFunc("/headpat/leaderboard/{user}", hdl.GetLeaderboardRankForUser)
	mux.Handle("POST /headpat/fulfill", hdl.AuthMiddleware(http.HandlerFunc(hdl.Fulfill)))

	fs := http.FileServer(http.Dir("client"))
	mux.Handle("/auth/", hdl.AuthMiddleware(RedirectIfAuth(fs)))
	mux.Handle("/control-panel/", hdl.AuthMiddleware(RequireAuth(fs)))
	mux.Handle("/overlay/", fs)
	mux.Handle("/favicon.ico", fs)
	mux.Handle("/favicon.svg", fs)
	mux.Handle("/favicon-96x96.png", fs)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
		default:
			http.NotFound(w, r)
		}
	})

	return &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
}

func RedirectIfAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler.HasAuth(r) && r.URL.Path == "/auth/" {
			http.Redirect(w, r, "/control-panel/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !handler.HasAuth(r) {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
