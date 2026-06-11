// Package config loads and validates the MCP server configuration from env vars.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the validated runtime configuration. Fields are ordered
// largest-alignment-first to satisfy the fieldalignment linter.
//
// Temperature is intentionally not configurable: captcha solving is fixed at
// temperature 0 for deterministic results (see the LLM provider request bodies).
type Config struct {
	Provider   string
	BaseURL    string
	APIKey     string
	Model      string
	AudioModel string
	HTTPPath   string
	LogLevel   string
	Transport  string
	HTTPAddr   string
	Timeout    time.Duration
	MaxTokens  int
}

var validProviders = map[string]bool{
	"anthropic": true, "openai": true, "openai-compatible": true, "gemini": true,
}

var validTransports = map[string]bool{
	"stdio": true, "http": true,
}

// Load reads CAPTCHA_LLM_* and CAPTCHA_TRANSPORT_* env vars and returns a validated Config.
func Load() (Config, error) {
	c := Config{
		Provider:   os.Getenv("CAPTCHA_LLM_PROVIDER"),
		BaseURL:    os.Getenv("CAPTCHA_LLM_BASE_URL"),
		APIKey:     os.Getenv("CAPTCHA_LLM_API_KEY"),
		Model:      os.Getenv("CAPTCHA_LLM_MODEL"),
		AudioModel: os.Getenv("CAPTCHA_LLM_AUDIO_MODEL"),
		LogLevel:   envOr("CAPTCHA_LOG_LEVEL", "info"),
		Transport:  envOr("CAPTCHA_TRANSPORT", "stdio"),
		HTTPAddr:   envOr("CAPTCHA_HTTP_ADDR", ":8080"),
		HTTPPath:   envOr("CAPTCHA_HTTP_PATH", "/mcp"),
		MaxTokens:  1024,
		Timeout:    60 * time.Second,
	}
	if !validProviders[c.Provider] {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_PROVIDER must be one of anthropic|openai|openai-compatible|gemini, got %q", c.Provider)
	}
	if !validTransports[c.Transport] {
		return Config{}, fmt.Errorf("CAPTCHA_TRANSPORT must be one of stdio|http, got %q", c.Transport)
	}
	if c.APIKey == "" && c.BaseURL == "" {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_API_KEY is required (or set CAPTCHA_LLM_BASE_URL for a keyless local endpoint)")
	}
	if c.Model == "" {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_MODEL is required")
	}
	if c.Transport == "http" {
		if c.HTTPPath == "" || c.HTTPPath[0] != '/' {
			return Config{}, fmt.Errorf("CAPTCHA_HTTP_PATH must be an absolute path beginning with '/', got %q", c.HTTPPath)
		}
	}
	if v := os.Getenv("CAPTCHA_LLM_MAX_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("CAPTCHA_LLM_MAX_TOKENS: %w", err)
		}
		c.MaxTokens = n
	}
	if v := os.Getenv("CAPTCHA_LLM_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("CAPTCHA_LLM_TIMEOUT: %w", err)
		}
		c.Timeout = d
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
