// Package config loads service settings from the environment (and optional .env).
package config

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// Config holds runtime settings for the OpenAI-compatible client and server.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Addr    string
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

// LoadDotEnv loads a local .env if present. Process env vars always win.
func LoadDotEnv() {
	if err := godotenv.Load(); err != nil {
		log.Printf("[config] no .env file loaded (%v) — using process environment / defaults", err)
		return
	}
	log.Println("[config] loaded .env (process env vars still override file values)")
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		BaseURL: envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   envOr("OPENAI_MODEL", string(openai.GPT4oMini)),
		Addr:    envOr("HTTP_ADDR", ":8080"),
	}
	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
