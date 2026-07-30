// Package logging configures stdout logging helpers used across the service.
package logging

import (
	"log"
	"os"
	"strings"
)

// Setup writes logs to stdout with microsecond timestamps.
func Setup() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("")
}

// RedactKey masks secrets for safe logging.
func RedactKey(key string) string {
	if key == "" {
		return "(empty)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

// Truncate shortens long strings for readable log lines.
func Truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
