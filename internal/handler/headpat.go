package handler

import (
	"encoding/json"
	"fmt"
	"headpat-counter/internal/client"
	"headpat-counter/internal/store"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

const keepaliveDuration time.Duration = 30 * time.Second

func (h *Handler) GetCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.st.GetHeadpatCount("headpat")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to get headpat count")
		log.Printf("Error: Failed to get headpat count: %s\n", err)
		return
	}
	message := client.HeadpatMessage{
		Count:     count.Pending,
		Total:     count.Total,
		Timestamp: string(time.Now().Format(time.RFC3339Nano)),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 {
		count = 10
	}

	scores, err := h.st.GetLeaderboard("headpat", count)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to get headpat leaderboard\n")
		log.Printf("Error: Failed to get headpat count: %s\n", err)
		return
	}

	n := len(scores)
	maxRankWidth := len(strconv.Itoa(int(scores[n-1].Rank)))
	maxScoreWidth := len(strconv.Itoa(int(scores[0].Score)))

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Top headpatters girldmHeadpat \n")
	for _, entry := range scores {
		fmt.Fprintf(w, "%*d. %*d - %s\n", maxRankWidth, entry.Rank, maxScoreWidth, int(entry.Score), entry.User)
	}
}

func (h *Handler) GetLeaderboardRankForUser(w http.ResponseWriter, r *http.Request) {
	userString := r.PathValue("user")
	score, err := h.st.GetScoreByUser("headpat", userString)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to get headpat rank\n")
		log.Printf("Error: Failed to get headpat rank: %s\n", err)
		return
	}

	if score.Rank == -1 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "no headpats girldmCrybaby \n")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "You are ranked %d with %d headpats redeemed girldmHeadpat \n", score.Rank, int(score.Score))
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	client := h.cm.NewClient()
	defer h.cm.CloseClient(client)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, _ := w.(http.Flusher)

	keepalive := time.NewTicker(keepaliveDuration)
	defer keepalive.Stop()

	for {
		select {
		case message := <-client:
			data, _ := json.Marshal(message)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			keepalive.Reset(keepaliveDuration)
		case <-keepalive.C:
			fmt.Fprint(w, ":\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) Fulfill(w http.ResponseWriter, r *http.Request) {
	if !HasAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "Unauthorized")
		return
	}

	type RequestBody struct {
		Amount int `json:"amount"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to read request")
		log.Printf("Error: Failed to read request: %s\n", err)
		return
	}

	var req RequestBody
	if err = json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad request")
		log.Printf("Error: Failed to parse payload: %s\n", err)
		return
	}

	count, err := h.st.FulfillEvent("headpat", req.Amount)
	if err == store.NoChange {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Error: Failed to fulfill headpats: %s\n", err)
		return
	}

	msg := client.HeadpatMessage{
		Count:     count.Pending,
		Total:     count.Total,
		Timestamp: string(time.Now().Format(time.RFC3339Nano)),
	}

	json.NewEncoder(w).Encode(msg)
	h.cm.SendAll(msg)
}
