package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"headpat-counter/internal/client"
	"headpat-counter/internal/poller"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

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
	EventType        string `json:"type"`
	StartedAt        string `json:"started_at"`
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

func (h *Handler) EventsubCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to read request")
		log.Printf("Error: Failed to read request: %s\n", err)
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
			fmt.Fprintf(w, "Failed to parse JSON")
			log.Printf("Error: Failed to parse webhook notifiation payload: %s\n", err)
			break
		}
		w.WriteHeader(http.StatusNoContent)
		flusher.Flush()
		h.handleNotification(message)

	case "webhook_callback_verification":
		type VerificationPayload struct {
			Subscription Subscription `json:"subscription"`
			Challenge    string       `json:"challenge"`
		}

		var message VerificationPayload
		if err = json.Unmarshal(body, &message); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to parse JSON")
			log.Printf("Error: Failed to parse webhook verification payload: %s\n", err)
			break
		}

		log.Println("Verifying subscription:", message.Subscription.Type)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, message.Challenge)

	case "revocation":
		type RevocationPayload struct {
			Subscription Subscription `json:"subscription"`
		}

		var message RevocationPayload
		if err = json.Unmarshal(body, &message); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to parse JSON")
			log.Printf("Error: Failed to parse webhook revocation payload: %s\n", err)
			break
		}

		w.WriteHeader(http.StatusNoContent)
		log.Println("Subscription revoked:", message.Subscription.Type, message.Subscription.Status)
	}
}

func (h *Handler) handleNotification(notification NotificationPayload) {
	switch notification.Subscription.Type {
	case "channel.channel_points_custom_reward_redemption.add":
		var event ChannelPointsRedemptionEvent
		if err := json.Unmarshal(notification.Event, &event); err != nil {
			log.Printf("Error: Failed to parse event: channel.channel_points_custom_reward_redemption.add: %s\n", err)
			break
		}

		if h.shouldAddHeadpat(event) {
			newCount, err := h.st.AddPendingHeadpat(event.Id)
			if err != nil {
				log.Printf("Error: Failed to add headpat event: %s\n", err)
				break
			}

			err = h.st.ScoreboardIncr("headpat", event.UserLogin, 1)
			if err != nil {
				log.Printf("Error: Failed to increment scoreboard: %s\n", err)
			}

			h.cm.SendAll(client.HeadpatMessage{
				Count:     newCount.Pending,
				Total:     newCount.Total,
				Timestamp: string(time.Now().Format(time.RFC3339Nano)),
			})
		}

	case "stream.online":
		var event StreamOnlineEvent
		if err := json.Unmarshal(notification.Event, &event); err != nil {
			log.Printf("Error: Failed to parse event: stream.online: %s\n", err)
			return
		}

		log.Printf("Stream online { id: %s }\n", event.Id)

		timestamp, err := time.Parse(time.RFC3339, event.StartedAt)
		if err != nil {
			log.Printf("Error: Failed to parse stream.online timestamp: %s\n", err)
			return
		}

		if err := h.st.AddStreamStartEvent(event.Id, timestamp); err != nil {
			log.Printf("Error: Failed to add stream start event: %s\n", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), h.cfg.PollerTimeout)
		defer cancel()

		poller.CurrentStreamId = event.Id
		headpatPoller := poller.NewHeadpatPoller(h.cfg, h.st, event.Id)
		headpatPoller.Start(ctx)

		// Add one final headpat message
		count, err := h.st.GetHeadpatCount()
		if err != nil {
			log.Printf("Error: Failed to get headpat count: %s\n", err)
			return
		}
		message := client.HeadpatMessage{
			Count:     count.Pending,
			Total:     count.Total,
			Timestamp: string(time.Now().Format(time.RFC3339Nano)),
		}
		h.cm.SendAll(message)

		// Reset stream headpats
		err = h.st.ResetScoreboard("stream:headpats")
		if err != nil {
			log.Printf("Error: failed to reset stream headpats: %s", err)
		}
	}
}

func (h *Handler) shouldAddHeadpat(event ChannelPointsRedemptionEvent) bool {
	// Skip notifications that are older than 10 minutes
	timestamp, _ := time.Parse(time.RFC3339Nano, event.RedeemedAt)
	if time.Since(timestamp) > 10*time.Minute {
		log.Println("Skipping event older than 10 minutes")
		return false
	}

	// Skip notifications that are not from headpats
	if event.BroadcasterId != h.cfg.BroadcasterId || event.Reward.Id != h.cfg.RewardId {
		log.Println("Skipping event that was not a headpat")
		return false
	}

	// Skip this headpat if it has already been counted
	if h.st.HeadpatExists(event.Id) {
		log.Println("Skipping headpat that was already recorded")
		return false
	}

	// Skip this headpat if the reward is out-of-stock
	if h.st.IsOutOfStock() {
		log.Println("Skipping headpat: already out of stock")
		return false
	}

	return true
}

func verifySignature(messageSignature, messageID, messageTimestamp string, body []byte) bool {
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(messageID + messageTimestamp))
	mac.Write(body)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(messageSignature))
}
