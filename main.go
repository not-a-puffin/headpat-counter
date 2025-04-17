package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// File to store count persistently
const saveFile = "save/total.json"

type HeadpatCount struct {
	StreamCount int `json:"count"`
	TotalCount  int `json:"total"`
}

type Headpat struct {
	UserID     string
	RedeemedAt string
	Rank       int
}

type AppState struct {
	mu sync.RWMutex

	// Store connected clients
	clients map[chan HeadpatCount]bool

	// Keep track of headpat events per stream to identify duplicates
	headpats map[string]Headpat

	// The ID of the current stream.
	// If there is no live stream, this should be the empty string
	streamID string

	// The headpat count for the current stream
	count int

	// The total headpat count
	total int

	isDirty bool
}

var state = AppState{}

// Add headpat to map and increment counter
func addHeadpat(eventID string, headpat Headpat) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.headpats[eventID] = headpat
	state.count++
	state.total++
	state.isDirty = true
}

func onStreamStart(streamID string) {
	state.mu.Lock()
	state.streamID = streamID
	state.mu.Unlock()
}

func onStreamEnd() {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.isDirty {
		saveToFile()
		state.isDirty = false
	}

	state.streamID = ""
	state.count = 0
	clear(state.headpats)
}

type SaveState struct {
	Total int `json:"total"`
}

func loadFromFile() {
	file, err := os.Open(saveFile)
	if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer file.Close()

	state.mu.Lock()
	defer state.mu.Unlock()

	decoder := json.NewDecoder(file)
	saveState := SaveState{}
	if err := decoder.Decode(&saveState); err != nil {
		log.Fatal("Error decoding save state:", err)
	}

	state.total = saveState.Total
}

func saveToFile() {
	file, err := os.Create(saveFile)
	if err != nil {
		log.Println("Error saving counter:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	saveState := SaveState{state.total}
	if err := encoder.Encode(saveState); err != nil {
		log.Println("Error encoding save state:", err)
	}

	log.Println("Saved to file")
}

func NewHeadpatCount() HeadpatCount {
	return HeadpatCount{
		StreamCount: state.count,
		TotalCount:  state.total,
	}
}

// Get the current headpat count
func getCount(c *gin.Context) {
	c.JSON(http.StatusOK, NewHeadpatCount())
}

func getEvents(c *gin.Context) {
	client := make(chan HeadpatCount)
	state.clients[client] = true

	defer func() {
		delete(state.clients, client)
		close(client)
	}()

	c.Stream(func(w io.Writer) bool {
		if count, ok := <-client; ok {
			message, _ := json.Marshal(count)
			c.SSEvent("message", string(message))
			return true
		}
		return false
	})
}

// Get static overlay HTML page
func getOverlay(c *gin.Context) {
	c.File("index.html")
}

// Before handling any message, we must make sure that it was sent by Twitch.
func verifySignature(messageSignature, messageID, messageTimestamp string, body []byte) bool {
	webhookSecret := os.Getenv("WEBHOOK_SECRET")

	// Create HMAC hash
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(messageID + messageTimestamp))
	mac.Write(body)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(messageSignature))
}

// Handle notifications from Twitch
func onWebhookEvent(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to read request body"})
		return
	}

	messageSignature := c.GetHeader("Twitch-Eventsub-Message-Signature")
	messageID := c.GetHeader("Twitch-Eventsub-Message-Id")
	messageTimestamp := c.GetHeader("Twitch-Eventsub-Message-Timestamp")

	if !verifySignature(messageSignature, messageID, messageTimestamp, body) {
		c.Status(http.StatusForbidden)
		return
	}

	var notification gin.H
	err = json.Unmarshal(body, &notification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to parse JSON"})
		return
	}

	messageType := c.GetHeader("Twitch-Eventsub-Message-Type")

	switch messageType {
	case "notification":
		c.AbortWithStatus(http.StatusNoContent)
		handleNotification(notification)

	case "webhook_callback_verification":
		challenge := notification["challenge"].(string)
		c.Data(http.StatusOK, "text/plain", []byte(challenge))

		subscription, _ := notification["subscription"]
		subscriptionType := subscription.(map[string]any)["type"].(string)
		log.Println("Verifying subscription: ", subscriptionType)

	case "revocation":
		c.AbortWithStatus(http.StatusNoContent)

		subscription, _ := notification["subscription"]
		subscriptionType := subscription.(map[string]any)["type"].(string)
		subscriptionStatus := subscription.(map[string]any)["status"].(string)
		log.Println("Subscription revoked: ", subscriptionType, subscriptionStatus)
	}
}

func handleNotification(notification map[string]any) {
	// Make sure notification has a "subscription" field
	subscription, ok := notification["subscription"]
	if !ok {
		log.Println("Unexpected notification")
		return
	}

	// Make sure notification has an "event" field
	event, ok := notification["event"]
	if !ok {
		log.Println("Unexpected notification")
		return
	}

	subscriptionType := subscription.(map[string]any)["type"].(string)

	switch subscriptionType {
	case "channel.channel_points_custom_reward_redemption.add":
		// Stop counting when 100 headpats has been reached
		if state.count >= 100 {
			break
		}

		// Skip headpat events if steam is not live
		if state.streamID == "" {
			break
		}

		// Skip notifications that are older than 10 minutes
		timestamp := event.(map[string]any)["redeemed_at"].(string)
		parsedTime, _ := time.Parse(time.RFC3339Nano, timestamp)
		duration := time.Since(parsedTime)
		if duration > 10*time.Minute {
			break
		}

		eventID := event.(map[string]any)["id"].(string)

		// Skip this headpat if it has already been counted
		_, ok := state.headpats[eventID]
		if ok {
			break
		}

		headpat := Headpat{
			UserID:     event.(map[string]any)["user_id"].(string),
			RedeemedAt: event.(map[string]any)["redeemed_at"].(string),
		}

		addHeadpat(eventID, headpat)

		// Send count to all clients
		for client := range state.clients {
			client <- NewHeadpatCount()
		}

		if state.count == 100 {
			onStreamEnd()
		}

	case "stream.online":
		log.Println("Stream online")

		streamID := event.(map[string]any)["id"].(string)
		onStreamStart(streamID)

		for client := range state.clients {
			client <- NewHeadpatCount()
		}

	case "stream.offline":
		log.Println("Stream offline")
		onStreamEnd()
	}
}

// Check occasionally if anything should be saved
func saveService() {
	for range time.Tick(time.Second * 900) {
		state.mu.Lock()
		if state.isDirty {
			saveToFile()
			state.isDirty = false
		}
		state.mu.Unlock()
	}
}

func main() {
	// Load environment variables
	godotenv.Load()

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.GET("/", getOverlay)
	router.GET("/count", getCount)
	router.GET("/events", getEvents)
	router.POST("/notification", onWebhookEvent)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Initialize state
	loadFromFile()
	state.clients = make(map[chan HeadpatCount]bool, 1)
	state.headpats = make(map[string]Headpat, 100)

	// Start save service as a goroutine
	go saveService()

	// Start server
	log.Println("Server started. Running on port", port)
	router.Run(":" + port)
}
