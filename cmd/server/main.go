// simple-ai-service: the smallest useful example of
//  1. a Go REST service
//  2. an AI "tool" (function) definition
//  3. using the OpenAI SDK to let the LLM call that tool
//
// Run:
//
//	cp .env.example .env   # optional
//	go run ./cmd/server
//
// Config via environment variables (.env supported):
//
//	OPENAI_BASE_URL   (default: https://api.openai.com/v1)
//	OPENAI_API_KEY    (optional — empty is fine for local runtimes)
//	OPENAI_MODEL      (default: gpt-4o-mini)
//	HTTP_ADDR         (default: :8080)
//
// Test:
//
//	curl -X POST localhost:8080/chat \
//	  -H "Content-Type: application/json" \
//	  -d '{"message":"What is the weather in Tokyo?"}'
package main

import (
	"log"

	"simple-ai-service/internal/config"
	"simple-ai-service/internal/handler"
	"simple-ai-service/internal/llm"
	"simple-ai-service/internal/logging"
	"simple-ai-service/internal/server"
	"simple-ai-service/internal/tools"
)

func main() {
	logging.Setup()

	log.Println("========== simple-ai-service starting ==========")

	config.LoadDotEnv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[config] OPENAI_BASE_URL=%s", cfg.BaseURL)
	log.Printf("[config] OPENAI_API_KEY=%s", logging.RedactKey(cfg.APIKey))
	log.Printf("[config] OPENAI_MODEL=%s", cfg.Model)
	log.Printf("[config] HTTP_ADDR=%s", cfg.Addr)

	client := llm.New(cfg.BaseURL, cfg.APIKey, cfg.Model)
	log.Printf("[client] OpenAI-compatible client ready base_url=%s model=%s", cfg.BaseURL, client.Model())
	log.Printf("[client] registered tools: %v — schemas offered on every /chat request", tools.Names())

	chat := &handler.Chat{LLM: client}
	srv := server.New(cfg.Addr, chat)
	log.Fatal(srv.ListenAndServe())
}
