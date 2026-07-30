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
	"strings"
	"time"

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

// redactKey masks secrets for safe logging.
func redactKey(key string) string {
	if key == "" {
		return "(empty)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

// truncate shortens long strings for readable log lines.
func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
	result := `{"location":"` + location + `","forecast":"sunny","tempC":24}`
	log.Printf("[tool] get_weather(location=%q) → %s", location, result)
	return result
}

// callTool dispatches a tool name to its Go implementation.
func callTool(name, argsJSON string) string {
	log.Printf("[tool] dispatch name=%q args=%s", name, truncate(argsJSON, 200))
	switch name {
	case "get_weather":
		var args struct {
			Location string `json:"location"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			log.Printf("[tool] failed to parse args for %q: %v", name, err)
			return `{"error":"invalid arguments"}`
		}
		return getWeather(args.Location)
	default:
		log.Printf("[tool] unknown tool %q — returning error payload", name)
		return `{"error":"unknown tool"}`
	}
}

// ---------- 3. OpenAI SDK: prompt the LLM, let it use the tool ----------

var client *openai.Client
var model string

func handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("[http] ← %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

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
	log.Printf("[chat] user message: %q", truncate(req.Message, 300))

	ctx := context.Background()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: req.Message},
	}

	// First call: give the model the tool and let it decide whether to use it.
	// Tool-calling support depends on the target provider and model — some
	// OpenAI-compatible servers ignore the tools field entirely. In that case
	// the model returns a plain text reply in the first turn and the tool loop
	// below is skipped, so the service still works for basic chat.
	log.Printf("[llm] round 1 → CreateChatCompletion model=%s tools=[get_weather] messages=1", model)
	t1 := time.Now()
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    []openai.Tool{getWeatherTool},
	})
	if err != nil {
		log.Printf("[llm] round 1 error after %s: %v", time.Since(t1).Round(time.Millisecond), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[llm] round 1 ← done in %s id=%s choices=%d",
		time.Since(t1).Round(time.Millisecond), resp.ID, len(resp.Choices))

	if len(resp.Choices) == 0 {
		log.Printf("[llm] round 1 returned no choices")
		http.Error(w, "model returned no choices", http.StatusInternalServerError)
		return
	}

	msg := resp.Choices[0].Message
	finish := resp.Choices[0].FinishReason
	log.Printf("[llm] round 1 finish_reason=%q content=%q tool_calls=%d",
		finish, truncate(msg.Content, 200), len(msg.ToolCalls))

	// If the model asked to call a tool, run it and send the result back.
	if len(msg.ToolCalls) > 0 {
		log.Printf("[flow] model requested %d tool call(s) — running local implementations", len(msg.ToolCalls))
		messages = append(messages, msg)

		for i, tc := range msg.ToolCalls {
			log.Printf("[flow] tool_call[%d] id=%s name=%s", i, tc.ID, tc.Function.Name)
			result := callTool(tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
			log.Printf("[flow] appended tool role message for tool_call_id=%s", tc.ID)
		}

		// Second call: model reads the tool result and writes a final answer.
		log.Printf("[llm] round 2 → CreateChatCompletion model=%s tools=none messages=%d (user + assistant + tool results)",
			model, len(messages))
		t2 := time.Now()
		resp, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		})
		if err != nil {
			log.Printf("[llm] round 2 error after %s: %v", time.Since(t2).Round(time.Millisecond), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(resp.Choices) == 0 {
			log.Printf("[llm] round 2 returned no choices")
			http.Error(w, "model returned no choices", http.StatusInternalServerError)
			return
		}
		msg = resp.Choices[0].Message
		log.Printf("[llm] round 2 ← done in %s id=%s finish_reason=%q content=%q",
			time.Since(t2).Round(time.Millisecond),
			resp.ID,
			resp.Choices[0].FinishReason,
			truncate(msg.Content, 300),
		)
	} else {
		log.Printf("[flow] no tool calls — using first-round content as the final reply")
	}

	log.Printf("[chat] final reply: %q", truncate(msg.Content, 400))
	writeJSON(w, chatResponse{Reply: msg.Content})
	log.Printf("[http] → 200 in %s", time.Since(start).Round(time.Millisecond))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[http] failed to write JSON response: %v", err)
	}
}

func main() {
	// Stdout + timestamps so learners can follow the flow in the terminal.
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("")

	log.Println("========== simple-ai-service starting ==========")

	// Load .env if present; real environment variables always take precedence.
	// Missing file is fine — config still comes from the process environment.
	if err := godotenv.Load(); err != nil {
		log.Printf("[config] no .env file loaded (%v) — using process environment / defaults", err)
	} else {
		log.Println("[config] loaded .env (process env vars still override file values)")
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[config] OPENAI_BASE_URL=%s", cfg.BaseURL)
	log.Printf("[config] OPENAI_API_KEY=%s", redactKey(cfg.APIKey))
	log.Printf("[config] OPENAI_MODEL=%s", cfg.Model)

	ocfg := openai.DefaultConfig(cfg.APIKey)
	ocfg.BaseURL = cfg.BaseURL
	client = openai.NewClientWithConfig(ocfg)
	model = cfg.Model
	log.Printf("[client] OpenAI-compatible client ready base_url=%s model=%s", cfg.BaseURL, model)
	log.Printf("[client] registered tool: get_weather — schema offered on every /chat request")

	http.HandleFunc("/chat", handleChat)
	log.Println("[http] route registered: POST /chat")
	log.Println("[http] listening on :8080")
	log.Println("========== ready — send a request to see the tool-calling flow ==========")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
