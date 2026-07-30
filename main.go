// simple-ai-service: the smallest useful example of
//  1. a Go REST service
//  2. an AI "tool" (function) definition
//  3. using the OpenAI SDK to let the LLM call that tool
//
// Run:
//
//	export OPENAI_API_KEY=sk-...
//	go run main.go
//
// Config via environment variables:
//
//	OPENAI_BASE_URL   (default: https://api.openai.com/v1)
//	OPENAI_API_KEY    (optional — empty is fine for local runtimes)
//	OPENAI_MODEL      (default: gpt-4o-mini)
//
// Test:
//
//	curl -X POST localhost:8080/chat \
//	  -H "Content-Type: application/json" \
//	  -d '{"message":"What is the weather in Tokyo?"}'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// ---------- Config ----------

type config struct {
	BaseURL string
	APIKey  string
	Model   string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid OPENAI_BASE_URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid OPENAI_BASE_URL %q: scheme must be http or https", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid OPENAI_BASE_URL %q: host is empty", raw)
	}
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		BaseURL: envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   envOr("OPENAI_MODEL", string(openai.GPT4oMini)),
	}
	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return config{}, err
	}
	return cfg, nil
}

// ---------- 1. REST types ----------

type chatRequest struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

// ---------- 2. Tool definition ----------
// This is the schema the model sees. It doesn't run any code itself;
// it just tells the LLM "here's a function you can ask me to call,
// and here's what arguments it needs."

var getWeatherTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "get_weather",
		Description: "Get the current weather for a given city",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {
					"type": "string",
					"description": "City name, e.g. Tokyo"
				}
			},
			"required": ["location"]
		}`),
	},
}

// The actual Go function behind the tool. In a real service this would
// call a weather API; here it just returns a fixed value so the example
// has zero external dependencies besides OpenAI itself.
func getWeather(location string) string {
	return `{"location":"` + location + `","forecast":"sunny","tempC":24}`
}

// callTool dispatches a tool name to its Go implementation.
func callTool(name, argsJSON string) string {
	switch name {
	case "get_weather":
		var args struct {
			Location string `json:"location"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		return getWeather(args.Location)
	default:
		return `{"error":"unknown tool"}`
	}
}

// ---------- 3. OpenAI SDK: prompt the LLM, let it use the tool ----------

var client *openai.Client
var model string

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: req.Message},
	}

	// First call: give the model the tool and let it decide whether to use it.
	// Tool-calling support depends on the target provider and model — some
	// OpenAI-compatible servers ignore the tools field entirely. In that case
	// the model returns a plain text reply in the first turn and the tool loop
	// below is skipped, so the service still works for basic chat.
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    []openai.Tool{getWeatherTool},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg := resp.Choices[0].Message

	// If the model asked to call a tool, run it and send the result back.
	if len(msg.ToolCalls) > 0 {
		messages = append(messages, msg)

		for _, tc := range msg.ToolCalls {
			result := callTool(tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// Second call: model reads the tool result and writes a final answer.
		resp, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg = resp.Choices[0].Message
	}

	writeJSON(w, chatResponse{Reply: msg.Content})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	// Load .env if present; real environment variables always take precedence.
	// Missing file is fine — config still comes from the process environment.
	_ = godotenv.Load()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ocfg := openai.DefaultConfig(cfg.APIKey)
	ocfg.BaseURL = cfg.BaseURL
	client = openai.NewClientWithConfig(ocfg)
	model = cfg.Model

	http.HandleFunc("/chat", handleChat)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
