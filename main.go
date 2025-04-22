package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"headpat-counter/store"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type HeadpatMessage struct {
	Count     int    `json:"count"`
	Total     int    `json:"total"`
	Timestamp string `json:"timestamp"`
}

type Reward struct {
	Id     string `json:"id"`
	Title  string `json:"title"`
	Cost   int    `json:"cost"`
	Prompt string `json:"prompt"`
}

type ChannelPointsRedemptionEvent struct {
	Id               string `json:"id"`
	BroadcasterId    string `json:"broadcaster_user_id"`
	BroadcasterLogin string `json:"broadcaster_user_login"`
	BroadcasterName  string `json:"broadcaster_user_name"`
	UserId           string `json:"user_id"`
	UserLogin        string `json:"user_login"`
	UserName         string `json:"user_name"`
	UserInput        string `json:"user_input"`
	Status           string `json:"status"`
	Reward           Reward `json:"reward"`
	RedeemedAt       string `json:"redeemed_at"`
}

type StreamOnlineEvent struct {
	Id               string `json:"id"`
	BroadcasterId    string `json:"broadcaster_user_id"`
	BroadcasterLogin string `json:"broadcaster_user_login"`
	BroadcasterName  string `json:"broadcaster_user_name"`
	StartedAt        string `json:"started_at"`
	Type             string `json:"type"`
}

type Subscription struct {
	Id        string          `json:"id"`
	Type      string          `json:"type"`
	Version   string          `json:"version"`
	Status    string          `json:"status"`
	Cost      int             `json:"cost"`
	Condition json.RawMessage `json:"condition"`
	Transport json.RawMessage `json:"transport"`
	CreatedAt string          `json:"created_at"`
}

type NotificationPayload struct {
	Subscription Subscription    `json:"subscription"`
	Event        json.RawMessage `json:"event"`
}

var (
	isDev         bool
	appClientId   string
	baseURL       string
	clientsMap    map[chan HeadpatMessage]bool
	clientsMutex  sync.RWMutex
	headpatsMap   map[string]bool
	headpatsMutex sync.RWMutex
	headpatCount  int
	headpatTotal  int
	broadcasterId string
	rewardId      string
	streamId      string
	memoryStore   store.Store
)

func newClient() chan HeadpatMessage {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	client := make(chan HeadpatMessage)
	clientsMap[client] = true
	return client
}

func closeClient(client chan HeadpatMessage) {
	log.Println("Closing client")

	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	delete(clientsMap, client)
	close(client)
}

func newHeadpatMessage() HeadpatMessage {
	return HeadpatMessage{
		Count:     headpatCount,
		Total:     headpatTotal,
		Timestamp: string(time.Now().Format(time.RFC3339Nano)),
	}
}

func tryAddHeadpat(event ChannelPointsRedemptionEvent) bool {
	headpatsMutex.Lock()
	defer headpatsMutex.Unlock()

	// if streamId == "" {
	// 	return false
	// }

	// Skip notifications that are older than 10 minutes
	timestamp, _ := time.Parse(time.RFC3339Nano, event.RedeemedAt)
	duration := time.Since(timestamp)
	if duration > 10*time.Minute {
		return false
	}

	// Skip notifications that are not from headpats
	if !isDev && (event.BroadcasterId != broadcasterId || event.Reward.Id != rewardId) {
		return false
	}

	// Skip this headpat if it has already been counted
	if _, ok := headpatsMap[event.Id]; ok {
		return false
	}

	headpatsMap[event.Id] = true
	headpatCount++
	headpatTotal++
	return true
}

func completeHeadpats(amount int) int {
	headpatsMutex.Lock()
	defer headpatsMutex.Unlock()

	if headpatCount > 0 {
		oldCount := headpatCount
		if amount == -1 {
			headpatCount = 0
			return oldCount
		}

		if amount > 0 {
			headpatCount -= amount
			if headpatCount < 0 {
				headpatCount = 0
			}
			return oldCount - headpatCount
		}
	}

	return 0
}

func sendHeadpats() {
	message := newHeadpatMessage()
	for client := range clientsMap {
		client <- message
	}
}

func verifySignature(messageSignature, messageID, messageTimestamp string, body []byte) bool {
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(messageID + messageTimestamp))
	mac.Write(body)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(messageSignature))
}

func handleNotification(notification NotificationPayload) {
	switch notification.Subscription.Type {
	case "channel.channel_points_custom_reward_redemption.add":
		var event ChannelPointsRedemptionEvent
		if err := json.Unmarshal(notification.Event, &event); err != nil {
			log.Println("Error: Failed to parse event: channel.channel_points_custom_reward_redemption.add")
			break
		}

		if ok := tryAddHeadpat(event); ok {
			sendHeadpats()
		}

	case "stream.online":
		log.Println("Stream online")

		var event StreamOnlineEvent
		if err := json.Unmarshal(notification.Event, &event); err != nil {
			log.Println("Error: Failed to parse event: stream.online")
			break
		}

		headpatsMutex.Lock()
		streamId = event.Id
		headpatsMutex.Unlock()

	case "stream.offline":
		log.Println("Stream offline")

		headpatsMutex.Lock()
		streamId = ""
		clear(headpatsMap)
		headpatsMutex.Unlock()
	}
}

func isValidToken(token string) bool {
	fmt.Println("Token:", token)
	session, _ := memoryStore.GetSession(token)
	return session != nil
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("AuthToken")
		isAuth := cookie != nil && isValidToken(cookie.Value)
		fmt.Println("isAuthenticated:", isAuth)
		ctx := context.WithValue(r.Context(), "isAuthenticated", isAuth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type OauthTokenResult struct {
	AccessToken  string          `json:"access_token"`
	ExpiresIn    int             `json:"expires_in"`
	RefreshToken string          `json:"refresh_token"`
	Scope        json.RawMessage `json:"scope"`
	TokenType    string          `json:"token_type"`
}

func getToken(code string) (*OauthTokenResult, error) {
	secret := os.Getenv("APP_CLIENT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("invalid client secret")
	}

	oauthURL := "https://id.twitch.tv/oauth2/token"
	params := url.Values{}
	params.Add("client_id", appClientId)
	params.Add("client_secret", secret)
	params.Add("code", code)
	params.Add("grant_type", "authorization_code")
	params.Add("redirect_uri", baseURL+"/auth/callback")
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

type UserLookupResult struct {
	Data []TwitchUser `json:"data"`
}

func lookupUser(accessToken string) (*TwitchUser, error) {
	helixURL := "https://api.twitch.tv/helix/users"

	req, err := http.NewRequest("GET", helixURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Client-Id", appClientId)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result UserLookupResult
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

func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token
}

func main() {
	isDev = os.Getenv("MODE") == "DEV"

	appClientId = os.Getenv("APP_CLIENT_ID")
	if appClientId == "" {
		log.Fatal("Missing client_id")
	}

	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Fatal("Missing base URL")
	}

	broadcasterId = os.Getenv("BROADCASTER_USER_ID")
	if !isDev && broadcasterId == "" {
		log.Fatal("Missing broadcaster_user_id")
	}

	rewardId = os.Getenv("REWARD_ID")
	if !isDev && broadcasterId == "" {
		log.Fatal("Missing reward_id")
	}

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("client"))
	mux.Handle("/overlay/", fs)
	mux.Handle("/favicon.ico", fs)
	mux.Handle("/favicon.svg", fs)
	mux.Handle("/favicon-96x96.png", fs)

	mux.Handle("/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth := r.Context().Value("isAuthenticated").(bool)
		if hasAuth {
			http.Redirect(w, r, "/control-panel/", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
		}
	})))

	mux.Handle("/auth/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth := r.Context().Value("isAuthenticated").(bool)
		if hasAuth {
			http.Redirect(w, r, "/control-panel/", http.StatusSeeOther)
		} else {
			http.ServeFile(w, r, "client/auth")
		}
	})))

	mux.HandleFunc("/auth/connect", func(w http.ResponseWriter, r *http.Request) {
		oauthURL := "https://id.twitch.tv/oauth2/authorize"
		params := url.Values{}
		params.Add("client_id", appClientId)
		params.Add("redirect_uri", baseURL+"/auth/callback")
		params.Add("response_type", "code")
		params.Add("scope", "channel:read:redemptions")
		url := oauthURL + "?" + params.Encode()
		http.Redirect(w, r, url, http.StatusSeeOther)
	})

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		defer http.Redirect(w, r, "/auth/", http.StatusSeeOther)

		code := r.URL.Query().Get("code")
		if code == "" {
			log.Println("Callback did not receive an authorization code")
			return
		}

		tokenResult, err := getToken(code)
		if tokenResult == nil || err != nil {
			log.Println("failed to get token:", err)
			return
		}

		user, err := lookupUser(tokenResult.AccessToken)
		if err != nil {
			log.Println("failed to lookup user:", err)
		}

		if !isDev && user.Id != broadcasterId {
			log.Println("Error: user is not girl_dm_")
		}

		sessionToken := generateSessionToken()
		session := store.Session{
			Access:  tokenResult.AccessToken,
			Refresh: tokenResult.RefreshToken,
		}
		if err = memoryStore.SetSession(sessionToken, session); err != nil {
			log.Println("Error: failed to save session:", err)
		}

		cookie := &http.Cookie{
			Name:     "AuthToken",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   !isDev,
			Expires:  time.Now().Add(24 * 60 * time.Hour),
		}
		http.SetCookie(w, cookie)
	})

	mux.Handle("/control-panel/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth := r.Context().Value("isAuthenticated").(bool)
		if hasAuth {
			http.ServeFile(w, r, "client/control-panel")
		} else {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
		}
	})))

	mux.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		message := newHeadpatMessage()
		json.NewEncoder(w).Encode(message)
	})

	mux.Handle("POST /complete-headpats", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth := r.Context().Value("isAuthenticated").(bool)
		if !hasAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type RequestBody struct {
			Amount int `json:"amount"`
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: Failed to read request")
			log.Printf("Error: Failed to read request at '/complete-headpats'")
			return
		}

		var req RequestBody
		if err = json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: Bad request")
			log.Println("Error: Failed to parse complete-headpats payload")
			return
		}

		completedCount := completeHeadpats(req.Amount)
		if completedCount > 0 {
			sendHeadpats()
			log.Printf("%d headpats completed!\n", completedCount)
		}
	})))

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		client := newClient()
		defer closeClient(client)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, _ := w.(http.Flusher)

		for {
			select {
			case message := <-client:
				data, _ := json.Marshal(message)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	mux.HandleFunc("POST /notification", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: Failed to read request")
			log.Printf("Error: Failed to read request at '/notification'")
			return
		}

		messageSignature := r.Header.Get("Twitch-Eventsub-Message-Signature")
		messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
		messageTimestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")

		if !verifySignature(messageSignature, messageID, messageTimestamp, body) {
			w.WriteHeader(http.StatusForbidden)
			log.Println("Failed to verify message")
			return
		}

		flusher, _ := w.(http.Flusher)

		messageType := r.Header.Get("Twitch-Eventsub-Message-Type")
		switch messageType {
		case "notification":
			var message NotificationPayload
			if err = json.Unmarshal(body, &message); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error: Failed to parse JSON")
				log.Println("Error: Failed to parse webhook notifiation payload")
				break
			}
			w.WriteHeader(http.StatusNoContent)
			flusher.Flush()
			handleNotification(message)

		case "webhook_callback_verification":
			type VerificationPayload struct {
				Subscription Subscription `json:"subscription"`
				Challenge    string       `json:"challenge"`
			}

			var message VerificationPayload
			if err = json.Unmarshal(body, &message); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error: Failed to parse JSON")
				log.Println("Error: Failed to parse webhook verification payload")
				break
			}

			log.Println("Verifying subscription:", message.Subscription.Type)
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, message.Challenge)

		case "revocation":
			type RevocationPayload struct {
				Subscription Subscription `json:"subscription"`
			}

			w.WriteHeader(http.StatusNoContent)

			var message RevocationPayload
			if err = json.Unmarshal(body, &message); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error: Failed to parse JSON")
				log.Println("Error: Failed to parse webhook revocation payload")
				break
			}

			w.WriteHeader(http.StatusNoContent)
			log.Println("Subscription revoked:", message.Subscription.Type, message.Subscription.Status)
		}
	})

	clientsMap = make(map[chan HeadpatMessage]bool, 1)
	headpatsMap = make(map[string]bool, 100)

	if isDev {
		memoryStore = store.NewMemoryStore()
	} else {
		memoryStore = store.NewRedisStore("localhost:6379")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	fmt.Println("Server listening on port", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
