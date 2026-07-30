// Package server wires HTTP routes and starts the listener.
package server

import (
	"log"
	"net/http"

	"simple-ai-service/internal/handler"
)

// Server is the HTTP front door for the service.
type Server struct {
	addr string
	mux  *http.ServeMux
}

// New registers routes on a dedicated ServeMux.
func New(addr string, chat *handler.Chat) *Server {
	mux := http.NewServeMux()
	mux.Handle("/chat", chat)
	return &Server{addr: addr, mux: mux}
}

// ListenAndServe starts the HTTP server (blocking).
func (s *Server) ListenAndServe() error {
	log.Printf("[http] route registered: POST /chat")
	log.Printf("[http] listening on %s", s.addr)
	log.Println("========== ready — send a request to see the tool-calling flow ==========")
	return http.ListenAndServe(s.addr, s.mux)
}
