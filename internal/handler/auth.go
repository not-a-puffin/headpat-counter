package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"headpat-counter/internal/request"
	"headpat-counter/internal/store"
	"log"
	"net/http"
	"net/url"
	"time"
)

type contextKey int

const (
	contextKeyAuth      contextKey    = iota + 1
	cookieLifetime      time.Duration = 24 * time.Hour * 180
	cookieRefreshWindow time.Duration = 24 * time.Hour * 30
)

func HasAuth(r *http.Request) bool {
	auth, ok := r.Context().Value(contextKeyAuth).(bool)
	return ok && auth
}

func (h *Handler) ConnectToTwitch(w http.ResponseWriter, r *http.Request) {
	oauthURL := "https://id.twitch.tv/oauth2/authorize"
	params := url.Values{}
	params.Add("client_id", h.cfg.AppClientId)
	params.Add("redirect_uri", h.cfg.BaseURL+"/auth/callback")
	params.Add("response_type", "code")
	params.Add("scope", "channel:read:redemptions")
	url := oauthURL + "?" + params.Encode()
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func (h *Handler) AuthCallback(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/auth/", http.StatusSeeOther)

	log.Println("Received auth callback")

	code := r.URL.Query().Get("code")
	if code == "" {
		log.Println("Callback did not receive an authorization code")
		return
	}

	tokenResult, err := request.GetUserAccessToken(h.cfg, code)
	if tokenResult == nil || err != nil {
		log.Printf("Error: failed to get user access token: %s\n", err)
		return
	}

	user, err := request.GetTwitchUser(h.cfg, tokenResult.AccessToken)
	if err != nil {
		log.Printf("Error: failed to lookup user: %s\n", err)
		return
	}

	if h.cfg.IsDev() && user.Id != h.cfg.BroadcasterId {
		log.Println("Error: user is not girl_dm_")
		return
	}

	log.Println("Setting session cookie")
	sessionToken := generateSessionToken()
	cookie := h.createCookie(sessionToken)
	http.SetCookie(w, &cookie)

	session := store.Session{
		UserId:  user.Id,
		Expires: cookie.Expires,
	}
	if err = h.st.SetSession(sessionToken, session); err != nil {
		log.Printf("Error: failed to save session: %s\n", err)
	}

	tokenPair := store.TokenPair{
		Access:  tokenResult.AccessToken,
		Refresh: tokenResult.RefreshToken,
	}
	if err = h.st.SetTokenPair("user", tokenPair); err != nil {
		log.Printf("Error: failed to save user access token pair: %s\n", err)
	}
}

func (h *Handler) IsAuthorized(w http.ResponseWriter, r *http.Request) bool {
	cookie, _ := r.Cookie(h.cfg.CookieName)
	if cookie == nil {
		// log.Println("Unauthorized: No session cookie")
		return false
	}

	session, _ := h.st.GetSession(cookie.Value)
	if session == nil {
		// log.Println("Unauthorized: No active sessions found")
		return false
	}

	log.Printf("Found session: { user: %s, expires: %s } ", session.UserId, session.Expires.Format(time.RFC3339))
	if time.Until(session.Expires) < cookieRefreshWindow {
		log.Println("Refreshing session cookie")
		cookie := h.createCookie(cookie.Value)
		http.SetCookie(w, &cookie)
	}

	return true
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth := h.IsAuthorized(w, r)
		ctx := context.WithValue(r.Context(), contextKeyAuth, hasAuth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token
}

func (h *Handler) createCookie(value string) http.Cookie {
	return http.Cookie{
		Name:     h.cfg.CookieName,
		Path:     "/",
		Value:    value,
		HttpOnly: true,
		Secure:   !h.cfg.IsDev(),
		Expires:  time.Now().Add(cookieLifetime),
	}
}
