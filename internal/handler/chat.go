// Package handler exposes HTTP endpoints for the service.
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"simple-ai-service/internal/logging"
)

// Chatter produces a reply for a user message (typically the LLM + tools loop).
type Chatter interface {
	Chat(ctx context.Context, userMessage string) (string, error)
}

// Chat handles POST /chat.
type Chat struct {
	LLM Chatter
}

type chatRequest struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

// ServeHTTP implements http.Handler.
func (h *Chat) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("[http] ← %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("[http] reject: method %s not allowed", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[http] reject: invalid JSON body: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		log.Printf("[http] reject: empty message field")
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	log.Printf("[chat] user message: %q", logging.Truncate(req.Message, 300))

	reply, err := h.LLM.Chat(r.Context(), req.Message)
	if err != nil {
		log.Printf("[chat] failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[chat] final reply: %q", logging.Truncate(reply, 400))
	writeJSON(w, chatResponse{Reply: reply})
	log.Printf("[http] → 200 in %s", time.Since(start).Round(time.Millisecond))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[http] failed to write JSON response: %v", err)
	}
}
